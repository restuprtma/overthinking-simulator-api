// Package meta_consumer wires the JetStream subscriber for the
// low-volume "market metadata" record types (Control 13, Top 20 17,
// Activity 39, Trading Status 57, Trading Summary 130) into a single
// Redis state cache.
//
// All five types share one consumer because their combined throughput
// is <100 msg/s and they all write to the same Redis instance — running
// five separate services would be boilerplate without operational
// benefit. NBS (Type 58/59) is intentionally NOT included here; it has
// its own dedicated consumer because the analytics use-case differs.
package meta_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/meta_consumer/service"
	"tuai/internal/modules/stock/meta_consumer/sink"
	"tuai/internal/modules/stock/meta_consumer/subscriber"
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

// Initialize constructs all dependencies and pings Redis up-front
// (fail-fast). NATS connect happens later inside Run().
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
