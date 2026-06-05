package sink

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tuai/internal/modules/stock/running_trade_consumer/trade"

	qdb "github.com/questdb/go-questdb-client/v3"
)

// TickWriter writes a single raw trade tick to durable storage.
// Implemented by QuestDBSink — kept as an interface so future durable
// sinks (Parquet archiver, ClickHouse, etc.) can plug in.
type TickWriter interface {
	Write(ctx context.Context, t trade.Trade) error
	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// QuestDBConfig holds the QuestDB ILP connection params. Uses the HTTP
// transport — TCP available too if you need lower latency, but HTTP is
// stateless and survives restarts cleanly.
type QuestDBConfig struct {
	Address           string        // e.g. localhost:9000 (HTTP ILP port)
	Table             string        // default "running_trades"
	Token             string        // optional bearer token
	BasicAuthUser     string        // alternative to token
	BasicAuthPassword string        //
	AutoFlushRows     int           // default 1000
	AutoFlushInterval time.Duration // default 500ms
}

// DefaultQuestDBConfig returns sane batch defaults for IDX trade volume.
func DefaultQuestDBConfig() QuestDBConfig {
	return QuestDBConfig{
		Table:             "running_trades",
		AutoFlushRows:     1000,
		AutoFlushInterval: 500 * time.Millisecond,
	}
}

// QuestDBSink writes raw ticks to QuestDB via ILP.
//
// The sender is owned by this sink and is goroutine-safe via a mutex —
// QuestDB's LineSender itself is NOT concurrent-safe.
type QuestDBSink struct {
	cfg    QuestDBConfig
	sender qdb.LineSender
	mu     sync.Mutex
}

// NewQuestDB connects to QuestDB and verifies the sender can be created.
// Does not write a row — that happens later via Write(). To fail fast on
// bad address, the Phase 1 caller should manually flush an empty buffer
// after construction (cheap noop).
func NewQuestDB(ctx context.Context, cfg QuestDBConfig) (*QuestDBSink, error) {
	if cfg.Table == "" {
		cfg.Table = "running_trades"
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

// Write inserts one trade tick. Auto-flush handles batching; manual flush
// also available via Flush() for graceful shutdown.
//
// Schema (matches docs/infra/topology.md §5.3):
//
//	timestamp      TIMESTAMP    (designated; UTC)
//	stock          SYMBOL       (indexed)
//	market         SYMBOL       (RG/TN/NG — set by consumer config)
//	command        LONG         (0=matched, 1=withdrawn — from Type 15)
//	price          DOUBLE
//	volume         LONG
//	buyer          SYMBOL       ("--" during live, real broker on resend)
//	seller         SYMBOL
//	buyer_type     SYMBOL       ("F" or "D")
//	seller_type    SYMBOL
//	buyer_order_no LONG
//	seller_order_no LONG
//	trade_no       LONG
//
// QuestDB ILP auto-creates missing columns on first write, but for
// production the schema should be ALTER TABLEd explicitly so column types
// are deterministic — see docs/infra/topology.md §5.3.
func (q *QuestDBSink) Write(ctx context.Context, t trade.Trade) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sender.
		Table(q.cfg.Table).
		Symbol("stock", t.Code).
		Symbol("market", t.Market).
		Symbol("buyer", t.Buyer).
		Symbol("seller", t.Seller).
		Symbol("buyer_type", t.BuyerType).
		Symbol("seller_type", t.SellerType).
		Int64Column("command", int64(t.Command)).
		Float64Column("price", t.Price).
		Int64Column("volume", t.Volume).
		Int64Column("buyer_order_no", t.BuyerOrderNo).
		Int64Column("seller_order_no", t.SellerOrderNo).
		Int64Column("trade_no", t.TradeNumber).
		StringColumn("date", t.RawDate).
		StringColumn("time", t.RawTime).
		At(ctx, t.Timestamp)
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
