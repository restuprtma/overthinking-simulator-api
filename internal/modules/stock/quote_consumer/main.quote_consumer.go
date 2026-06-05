// Package quote_consumer wires the JetStream subscriber for idx.quote.>
// to a Redis hash sink that stores the latest known state per stock
// (every IQPlus FID — prev_close, last_price, open, high, low, etc).
//
// Per docs/infra/topology.md §5.2 (Quote State Updater):
//   - Subscribe `idx.quote.>` (Type 14)
//   - Output: Redis hash `quote:<stock>` with named FID aliases + raw FID nums
//   - Reset at pre-open (~08:45 WIB) — TTL 25h handles eviction naturally
package quote_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/quote_consumer/service"
	"tuai/internal/modules/stock/quote_consumer/sink"
	"tuai/internal/modules/stock/quote_consumer/subscriber"
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

// Initialize constructs all dependencies and connects to Redis up-front
// (fail-fast on bad config). NATS connect happens later inside Run().
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
