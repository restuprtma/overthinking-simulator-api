// Package news_consumer wires the JetStream subscriber for idx.news.>
// to a MongoDB sink. Multi-packet news frames are reassembled in memory
// before insert.
//
// Per docs/infra/topology.md §5.5 (News Indexer):
//   - Subscribe `idx.news.>` (Type 36)
//   - Reassemble multi-packet → single document
//   - Output: MongoDB collection `news` with text index
package news_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/news_consumer/assembler"
	"tuai/internal/modules/stock/news_consumer/service"
	"tuai/internal/modules/stock/news_consumer/sink"
	"tuai/internal/modules/stock/news_consumer/subscriber"
)

// Module bundles the long-lived components.
type Module struct {
	Subscriber *subscriber.Subscriber
	Sink       *sink.MongoSink
	Assembler  *assembler.Assembler
	Service    *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber subscriber.Config
	Mongo      sink.Config
	Assembler  assembler.Config
	Service    service.Config
}

// Initialize constructs all dependencies and pings MongoDB up-front
// (fail-fast). NATS connect happens later inside Run().
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	mongoSink, err := sink.New(ctx, cfg.Mongo)
	if err != nil {
		return nil, fmt.Errorf("init mongo sink: %w", err)
	}

	asm := assembler.New(cfg.Assembler)

	svc := service.New(mongoSink, asm, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber: sub,
		Sink:       mongoSink,
		Assembler:  asm,
		Service:    svc,
	}, nil
}

// Run blocks until ctx is cancelled.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
