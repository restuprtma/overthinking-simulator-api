// Package tradedone_consumer wires the JetStream subscriber for
// idx.tradedone.> to a Redis volume-profile sink.
//
// Per docs/infra/topology.md §6.1 (Type 40 — Trade Done):
//   - Subscribe `idx.tradedone.>` (record type 40)
//   - Output: Redis hash `tradedone:<stock>` keyed by price level,
//     value is a JSON object with cumulative buy/sell/foreign volumes
//     + frequencies — overwritten on every vendor update.
//
// Use case: volume-profile chart per saham (volume distribution at each
// price level throughout the trading session).
package tradedone_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/tradedone_consumer/service"
	"tuai/internal/modules/stock/tradedone_consumer/sink"
	"tuai/internal/modules/stock/tradedone_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	Sink       *sink.RedisSink
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	Redis      sink.Config
	Service    service.Config
}

// Initialize constructs all dependencies and pings Redis up-front.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	redisSink, err := sink.New(ctx, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("init redis sink: %w", err)
	}

	svc := service.New(redisSink, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber: sub,
		Sink:       redisSink,
		Service:    svc,
	}, nil
}

// Run blocks until ctx is cancelled.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
