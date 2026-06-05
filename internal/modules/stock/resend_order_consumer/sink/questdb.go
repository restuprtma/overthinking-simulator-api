// Package sink writes parsed Resend Order rows to durable storage. The
// only implementation today is QuestDBSink (HTTP ILP). The OrderWriter
// interface lets future durable sinks (Parquet archiver, ClickHouse,
// etc.) plug in without touching the service.
package sink

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tuai/internal/modules/stock/orderbook_consumer/parser"

	qdb "github.com/questdb/go-questdb-client/v3"
)

// OrderWriter writes a single resend order row. Implemented by QuestDBSink.
type OrderWriter interface {
	Write(ctx context.Context, o parser.Order, market string, ts time.Time) error
	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// QuestDBConfig holds the QuestDB ILP connection params. Mirror of the
// running-trade-consumer / resend-trade-consumer sink config.
type QuestDBConfig struct {
	Address           string        // host:port (HTTP ILP, e.g. 10.10.8.51:9000)
	Table             string        // e.g. "orders"
	Token             string        // optional bearer token
	BasicAuthUser     string        // alternative to token
	BasicAuthPassword string        //
	AutoFlushRows     int           // default 1000
	AutoFlushInterval time.Duration // default 500ms
}

// DefaultQuestDBConfig returns sane batch defaults.
func DefaultQuestDBConfig() QuestDBConfig {
	return QuestDBConfig{
		Table:             "orders",
		AutoFlushRows:     1000,
		AutoFlushInterval: 500 * time.Millisecond,
	}
}

// QuestDBSink writes resend-order rows to QuestDB via ILP.
//
// The sender is owned by this sink and is goroutine-safe via a mutex —
// QuestDB's LineSender itself is NOT concurrent-safe.
type QuestDBSink struct {
	cfg    QuestDBConfig
	sender qdb.LineSender
	mu     sync.Mutex
}

// NewQuestDB connects to QuestDB and creates the line sender.
func NewQuestDB(ctx context.Context, cfg QuestDBConfig) (*QuestDBSink, error) {
	if cfg.Table == "" {
		cfg.Table = "orders"
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

// Write inserts one resend-order row.
//
// Schema:
//
//	timestamp     TIMESTAMP    (designated; UTC)
//	stock         SYMBOL       (indexed)
//	market        SYMBOL       (RG/TN/NG — derived from stock code suffix)
//	command       LONG         (0=bid, 1=offer, 2=cancel-bid, 3=cancel-offer)
//	order_no      LONG
//	price         DOUBLE
//	volume        LONG
//	broker        SYMBOL       (real broker code on resend)
//	balance       LONG         (vendor field; not always reliable)
//	investor      SYMBOL       ("F" or "D")
//	no_reference  LONG
//	date          STRING       (vendor raw envelope.Date "20260518" — caller sets o.RawDate)
//	time          STRING       (vendor raw o.Time "HHMMSS" WIB)
//
// QuestDB ILP auto-creates missing columns on first write, but for
// production the schema should be CREATE TABLEd explicitly with DEDUP
// UPSERT KEYS(timestamp, stock, order_no, command).
func (q *QuestDBSink) Write(ctx context.Context, o parser.Order, market string, ts time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sender.
		Table(q.cfg.Table).
		Symbol("stock", o.Stock).
		Symbol("market", market).
		Symbol("broker", o.Broker).
		Symbol("investor", o.Investor).
		Int64Column("command", int64(o.Command)).
		Int64Column("order_no", o.OrderNumber).
		Float64Column("price", o.Price).
		Int64Column("volume", o.Volume).
		Int64Column("balance", o.Balance).
		Int64Column("no_reference", o.NoReference).
		StringColumn("date", o.RawDate).
		StringColumn("time", o.Time).
		At(ctx, ts)
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
