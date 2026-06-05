// Package running_order_consumer wires the JetStream subscriber for
// idx.order.> (Type 16, live) to the QuestDB `running_orders` table.
//
// Per docs/iqplus/iqplus-data-feed-v4.0.0.md §1.6 (Type 16 — Order):
//
//   The vendor emits a Type 16 frame for every order add/cancel. During
//   live market hours, the Broker field is masked as "--" (regulation).
//   Real broker codes only appear in the post-close Type 26 (Resend
//   Order) batch — handled separately by cmd/resend-order-consumer →
//   table `orders`.
//
// This service is the live event log: every Type 16 row is written
// raw to `running_orders`. DEDUP UPSERT KEYS(timestamp, stock, order_no,
// command) guards against re-processed messages.
//
// Mirrors the running_trades half of cmd/running-trade-consumer for orders.
//
// Deps shared with orderbook-consumer / resend-order-consumer:
//   - parser: internal/modules/stock/orderbook_consumer/parser (Order, ResolveTimestamp)
//   - sink:   internal/modules/stock/resend_order_consumer/sink (QuestDBSink)
//   - market: internal/modules/stock/running_trade_consumer/trade (DeriveMarket)
package running_order_consumer

import (
	"context"
	"fmt"

	orderSink "tuai/internal/modules/stock/resend_order_consumer/sink"
	"tuai/internal/modules/stock/running_order_consumer/service"
	"tuai/internal/modules/stock/running_order_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	Sink       orderSink.OrderWriter
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	QuestDB    orderSink.QuestDBConfig
	Service    service.Config
}

// Initialize constructs the QuestDB sink and wires the service.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	qdb, err := orderSink.NewQuestDB(ctx, cfg.QuestDB)
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
