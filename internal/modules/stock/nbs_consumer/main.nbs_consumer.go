// Package nbs_consumer wires the JetStream subscriber for idx.nbs.>
// to QuestDB. Type 58 (NBS Stock) rows go to the `nbs_stock` table;
// Type 59 (NBS Broker) rows go to `nbs_broker`. Each table is a
// time-series of cumulative-during-day snapshots — use `LATEST ON`
// queries for current state, range scans for intraday flow.
//
// Per docs/infra/topology.md §5.7 (NBS Aggregator):
//   - Subscribe `idx.nbs.>` (Type 58 + Type 59)
//   - Output: QuestDB tables `nbs_stock` & `nbs_broker` (via ILP HTTP)
//
// Use case: bandar / foreign flow analytics — "broker mana akumulasi
// stock X?", "broker Y trading apa hari ini?"
package nbs_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/nbs_consumer/service"
	"tuai/internal/modules/stock/nbs_consumer/sink"
	"tuai/internal/modules/stock/nbs_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	Sink       sink.Writer
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	QuestDB    sink.QuestDBConfig
	Service    service.Config
}

// Initialize constructs all dependencies and connects to QuestDB up-front.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	qdbSink, err := sink.NewQuestDB(ctx, cfg.QuestDB)
	if err != nil {
		return nil, fmt.Errorf("init questdb sink: %w", err)
	}

	svc := service.New(qdbSink, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber: sub,
		Sink:       qdbSink,
		Service:    svc,
	}, nil
}

// Run blocks until ctx is cancelled.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
