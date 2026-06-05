// Package resend_order_consumer wires the JetStream subscriber for
// idx.resend.order.> to a single QuestDB table (`orders`).
//
// Per docs/iqplus/iqplus-data-feed-v4.0.0.md §5.7 (Type 26 — Resend Order):
//
//   During market hours, IDX masks broker codes in Type 16 (Order) as
//   "--". The vendor re-emits each order event as Type 26 (Resend Order)
//   with the real broker codes after market close.
//
//   This consumer subscribes to `idx.resend.order.>` and writes every
//   Type 26 row to the `orders` QuestDB table. DEDUP UPSERT KEYS
//   (timestamp, stock, order_no, command) on the table prevents
//   duplicates if a row is re-processed.
//
// Type 26 wire layout matches Type 16 (10 fields), but only HHMMSS is
// embedded — the date is taken from the envelope's frame Date stamp
// (with a one-day rollback when the vendor delivers post-midnight WIB —
// see service.resolveTimestamp).
//
// Deps shared with orderbook-consumer / resend-trade-consumer:
//   - parser: internal/modules/stock/orderbook_consumer/parser (Order)
//   - market: internal/modules/stock/running_trade_consumer/trade (DeriveMarket)
//
// Type 27 (Resend Trade) is NOT handled here — see cmd/resend-trade-consumer.
package resend_order_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/resend_order_consumer/service"
	"tuai/internal/modules/stock/resend_order_consumer/sink"
	"tuai/internal/modules/stock/resend_order_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	Sink       sink.OrderWriter
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	QuestDB    sink.QuestDBConfig
	Service    service.Config
}

// Initialize constructs the QuestDB sink and wires the service.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	qdb, err := sink.NewQuestDB(ctx, cfg.QuestDB)
	if err != nil {
		return nil, fmt.Errorf("init questdb sink: %w", err)
	}

	svc := service.New(qdb, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber: sub,
		Sink:       qdb,
		Service:    svc,
	}, nil
}

// Run blocks until ctx is cancelled.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
