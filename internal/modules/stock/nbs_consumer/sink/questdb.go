// Package sink writes Net Buy/Sell snapshots to QuestDB via ILP. Type 58
// (NBS Stock) rows go to the `nbs_stock` table, Type 59 (NBS Broker)
// rows go to `nbs_broker`. Both tables share the same column layout —
// the split keeps the vendor's two semantic views queryable as
// independent time series instead of conflating them into one table.
//
// Schema (canonical CREATE TABLE in deployments/kubernetes/nbs-consumer/README.md):
//
//	timestamp        TIMESTAMP    (designated; UTC midnight of trading day — day bucket)
//	stock            SYMBOL INDEX  (full code including board suffix, e.g. "WBSANG")
//	broker           SYMBOL INDEX  (e.g. "XL")
//	market           SYMBOL        (RG/NG/TN — derived from stock suffix, matches `trades.market`)
//	b_freq           LONG          (buy frequency)
//	b_vol            LONG          (buy volume — shares)
//	b_lot            LONG
//	b_val            LONG          (buy value — rupiah)
//	b_pct            DOUBLE
//	s_freq           LONG
//	s_vol            LONG
//	s_lot            LONG
//	s_val            LONG
//	s_pct            DOUBLE
//	sequence         LONG          (envelope.Sequence, for traceability)
//	date             STRING        (envelope.Date raw, "20260518" — vendor wall-clock day)
//	time             STRING        (envelope.Time raw, "171517"  — vendor wall-clock HHMMSS WIB)
//	last_updated_at  TIMESTAMP     (envelope.ReceivedAt UTC — actual snapshot receive precision)
//
// DEDUP UPSERT KEYS(timestamp, stock, broker) — vendor emits many snapshots
// per (stock, broker) per session, each cumulative. Truncating `timestamp`
// to UTC midnight day-bucket collapses all snapshots of one day for one
// pair into a single row; the latest INSERT wins (cumulative values are
// monotonically increasing, so latest = final EOD state). `last_updated_at`
// preserves the actual receive time of that latest snapshot.
//
// Storage reduction observed at first migration: 4.46M raw rows →
// 47K dedup'd rows (95× compression).
package sink

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tuai/internal/modules/stock/nbs_consumer/parser"

	qdb "github.com/questdb/go-questdb-client/v3"
)

// Writer is the abstraction the service depends on. Kept as an interface
// so future durable sinks (e.g. ClickHouse, Parquet archiver) can plug
// in without churn in service code.
type Writer interface {
	Apply(ctx context.Context, n parser.NBS, market, vendorDate, vendorTime string, ts time.Time, seq int64) error
	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// QuestDBConfig holds the QuestDB ILP connection params. Uses HTTP
// transport — stateless and survives restarts cleanly.
type QuestDBConfig struct {
	Address           string        // e.g. 10.10.8.51:9000 (HTTP ILP port)
	StockTable        string        // default "nbs_stock"   (Type 58 destination)
	BrokerTable       string        // default "nbs_broker"  (Type 59 destination)
	Token             string        // optional bearer token
	BasicAuthUser     string        // alternative to token
	BasicAuthPassword string        //
	AutoFlushRows     int           // default 1000
	AutoFlushInterval time.Duration // default 500ms
}

// DefaultQuestDBConfig returns sane batch defaults for NBS volume.
// NBS is bursty (large EOD snapshot dumps) — defaults match
// resend-trade-consumer which sees the same load shape.
func DefaultQuestDBConfig() QuestDBConfig {
	return QuestDBConfig{
		StockTable:        "nbs_stock",
		BrokerTable:       "nbs_broker",
		AutoFlushRows:     1000,
		AutoFlushInterval: 500 * time.Millisecond,
	}
}

// QuestDBSink routes every NBS row to its source-specific table. A
// single LineSender writes to both tables — ILP supports per-row
// Table() selection, no need for two connections.
type QuestDBSink struct {
	cfg    QuestDBConfig
	sender qdb.LineSender
	mu     sync.Mutex
}

// NewQuestDB constructs a QuestDB-backed sink. Does not perform a write
// here; first Apply or Flush surfaces address/auth errors.
func NewQuestDB(ctx context.Context, cfg QuestDBConfig) (*QuestDBSink, error) {
	if cfg.StockTable == "" {
		cfg.StockTable = "nbs_stock"
	}
	if cfg.BrokerTable == "" {
		cfg.BrokerTable = "nbs_broker"
	}
	if cfg.AutoFlushRows <= 0 {
		cfg.AutoFlushRows = 1000
	}
	if cfg.AutoFlushInterval <= 0 {
		cfg.AutoFlushInterval = 500 * time.Millisecond
	}

	opts := []qdb.LineSenderOption{
		qdb.WithHttp(),
		qdb.WithAddress(cfg.Address),
		qdb.WithAutoFlushRows(cfg.AutoFlushRows),
		qdb.WithAutoFlushInterval(cfg.AutoFlushInterval),
	}
	if cfg.Token != "" {
		opts = append(opts, qdb.WithBearerToken(cfg.Token))
	} else if cfg.BasicAuthUser != "" {
		opts = append(opts, qdb.WithBasicAuth(cfg.BasicAuthUser, cfg.BasicAuthPassword))
	}

	sender, err := qdb.NewLineSender(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("questdb sender: %w", err)
	}

	return &QuestDBSink{cfg: cfg, sender: sender}, nil
}

// Apply writes one NBS snapshot to the source-appropriate table.
// Source==58 → StockTable, Source==59 → BrokerTable. `market` is the
// IDX board code derived from the stock suffix (RG/NG/TN) — kept as a
// separate column so queries can filter by board without LIKE patterns
// on the symbol. Same convention as `trades.market`.
//
// `vendorDate` / `vendorTime` are the raw envelope.Date / envelope.Time
// strings (vendor wall-clock, WIB, no TZ marker) — preserved for audit
// and human-readable display. `ts` is the publisher receive time (UTC).
// The designated `timestamp` column is set to the UTC midnight of `ts`'s
// day (day-bucket) so DEDUP UPSERT KEYS(timestamp, stock, broker)
// collapses all snapshots of the same (day, stock, broker) into one row;
// `last_updated_at` preserves the actual `ts` precision for ordering and
// "kapan latest update" queries.
func (q *QuestDBSink) Apply(ctx context.Context, n parser.NBS, market, vendorDate, vendorTime string, ts time.Time, seq int64) error {
	if n.Stock == "" || n.Broker == "" {
		return nil
	}

	table := q.cfg.StockTable
	if n.Source == 59 {
		table = q.cfg.BrokerTable
	}

	dayBucket := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)

	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sender.
		Table(table).
		Symbol("stock", n.Stock).
		Symbol("broker", n.Broker).
		Symbol("market", market).
		Int64Column("b_freq", n.BFreq).
		Int64Column("b_vol", n.BVol).
		Int64Column("b_lot", n.BLot).
		Int64Column("b_val", n.BVal).
		Float64Column("b_pct", n.BPct).
		Int64Column("s_freq", n.SFreq).
		Int64Column("s_vol", n.SVol).
		Int64Column("s_lot", n.SLot).
		Int64Column("s_val", n.SVal).
		Float64Column("s_pct", n.SPct).
		Int64Column("sequence", seq).
		StringColumn("date", vendorDate).
		StringColumn("time", vendorTime).
		TimestampColumn("last_updated_at", ts).
		At(ctx, dayBucket)
}

// Flush forces any buffered rows out to QuestDB.
func (q *QuestDBSink) Flush(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sender.Flush(ctx)
}

// Shutdown flushes the buffer and closes the underlying transport.
func (q *QuestDBSink) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.sender == nil {
		return nil
	}
	return q.sender.Close(ctx)
}
