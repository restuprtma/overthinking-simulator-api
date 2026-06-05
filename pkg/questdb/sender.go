package questdb

import (
	"context"
	"fmt"
	"sync"

	qdb "github.com/questdb/go-questdb-client/v3"
)

// Sender is a thread-safe wrapper around qdb.LineSender. The
// underlying LineSender is single-goroutine per the upstream docs;
// callers that share one Sender across goroutines must do so through
// this type so the mutex enforces the constraint.
//
// Sender writes are zero-copy: Row(fn) hands you the raw LineSender
// under the lock, you build the chain, and the lock releases after
// At/AtNow returns. Auto-flush (by row count or interval) happens
// inside the LineSender — Flush() forces a manual flush, Close()
// flushes and shuts down the transport.
//
// One Sender can write to many tables — pick the destination per
// row with the .Table() call in your builder.
type Sender struct {
	inner qdb.LineSender
	mu    sync.Mutex
}

// NewSender opens an ILP HTTP connection to QuestDB using the
// settings in cfg. Construction itself is cheap; the first real
// network round-trip happens on the first auto-flush (or explicit
// Flush). Call Sender.Flush(ctx) after NewSender if you want to
// fail fast on a bad address.
func NewSender(ctx context.Context, cfg Config) (*Sender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := []qdb.LineSenderOption{
		qdb.WithHttp(),
		qdb.WithAddress(cfg.Address),
		qdb.WithAutoFlushRows(cfg.AutoFlushRows),
		qdb.WithAutoFlushInterval(cfg.AutoFlushInterval),
	}
	if cfg.TLS {
		opts = append(opts, qdb.WithTls())
	}
	if cfg.Token != "" {
		opts = append(opts, qdb.WithBearerToken(cfg.Token))
	} else if cfg.BasicAuthUser != "" {
		opts = append(opts, qdb.WithBasicAuth(cfg.BasicAuthUser, cfg.BasicAuthPassword))
	}

	inner, err := qdb.NewLineSender(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("questdb: open sender: %w", err)
	}
	return &Sender{inner: inner}, nil
}

// Row runs fn under the sender's mutex. fn receives the raw
// LineSender and must terminate the chain with At(ctx, ts) or
// AtNow(ctx); the LineSender's builder methods all return the
// same receiver so the chain is fluent.
//
// Example:
//
//	err := s.Row(func(ls qdb.LineSender) error {
//	    return ls.
//	        Table("running_trades").
//	        Symbol("stock", t.Code).
//	        Float64Column("price", t.Price).
//	        Int64Column("volume", t.Volume).
//	        At(ctx, t.Timestamp)
//	})
//
// Returning an error from fn does not abort the whole batch — the
// LineSender's internal buffer is not transactional — but it lets
// the caller surface schema mismatches early.
func (s *Sender) Row(fn func(qdb.LineSender) error) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("questdb: sender is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.inner)
}

// Flush forces buffered rows to QuestDB. Safe to call on a closed
// sender (returns the underlying error). Call from a ticker for
// services that want a max-latency guarantee independent of
// auto-flush thresholds.
func (s *Sender) Flush(ctx context.Context) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("questdb: sender is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Flush(ctx)
}

// Close flushes the buffer and releases the underlying HTTP
// connection. Idempotent — safe to defer Close right after
// NewSender succeeds.
func (s *Sender) Close(ctx context.Context) error {
	if s == nil || s.inner == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.inner.Close(ctx)
	s.inner = nil
	return err
}
