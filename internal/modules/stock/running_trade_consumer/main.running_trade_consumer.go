// Package running_trade_consumer wires the JetStream subscriber, trade parser,
// aggregator, and storage sinks for the IDX OHLCV pipeline.
//
// Phase 1 scope (per docs/infra/topology.md §7.1):
//   - Subscribe to idx.trade.>
//   - 1-minute bars only
//   - Live bar → Redis (key: candle:<stock>:1m)
//   - Raw tick → QuestDB (table: running_trades, via ILP HTTP)
//
// Future timeframes / closed-bar materialization belong in a separate
// downstream service that reads from QuestDB — not in this hot path.
package running_trade_consumer

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/running_trade_consumer/aggregator"
	"tuai/internal/modules/stock/running_trade_consumer/service"
	"tuai/internal/modules/stock/running_trade_consumer/sink"
	"tuai/internal/modules/stock/running_trade_consumer/subscriber"
)

// Module bundles the long-lived components.
//
// CandlePublisher is optional: when nil, candle updates only land in Redis
// (and downstream WS clients lose live broadcast). Set CandlePublishURL in
// the env to enable.
type Module struct {
	Subscriber      *subscriber.Subscriber
	TickWriter      sink.TickWriter
	LiveSink        *sink.RedisSink
	CandlePublisher *sink.NATSCandleSink
	Aggregator      *aggregator.Aggregator
	Service         *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	Subscriber     subscriber.Config
	QuestDB        sink.QuestDBConfig
	Redis          sink.RedisConfig
	NATSCandle     sink.NATSCandleConfig // empty URL → publisher disabled
	Aggregator     aggregator.Config
	Service        service.Config
	EnableNATSCand bool                  // explicit on-switch; default false
}

// Initialize constructs all dependencies and connects to Redis + QuestDB
// (and optionally the candle-broadcast NATS connection) up-front so the
// binary fails fast on bad config. The trade-feed JetStream consumer
// connects later inside Run() so we don't try to consume before sinks
// are ready.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	tickWriter, err := sink.NewQuestDB(ctx, cfg.QuestDB)
	if err != nil {
		return nil, fmt.Errorf("init questdb sink: %w", err)
	}

	liveSink, err := sink.NewRedis(ctx, cfg.Redis)
	if err != nil {
		_ = tickWriter.Shutdown(ctx)
		return nil, fmt.Errorf("init redis sink: %w", err)
	}

	// Aggregator sinks: Redis for live state, optional NATS for broadcast.
	// Order matters because aggregator.fanoutUpdate stops on first error —
	// Redis comes first because it's the source-of-truth for snapshots,
	// NATS-publish is best-effort and never returns errors anyway.
	aggSinks := []aggregator.Sink{liveSink}

	var candlePub *sink.NATSCandleSink
	if cfg.EnableNATSCand && cfg.NATSCandle.URL != "" {
		candlePub, err = sink.NewNATSCandle(ctx, cfg.NATSCandle)
		if err != nil {
			_ = tickWriter.Shutdown(ctx)
			_ = liveSink.Shutdown()
			return nil, fmt.Errorf("init nats candle publisher: %w", err)
		}
		aggSinks = append(aggSinks, candlePub)
	}

	agg := aggregator.New(cfg.Aggregator, aggSinks...)

	svc := service.New(tickWriter, agg, cfg.Service)
	sub := subscriber.New(cfg.Subscriber, svc.Handler())
	svc.AttachSubscriber(sub)

	return &Module{
		Subscriber:      sub,
		TickWriter:      tickWriter,
		LiveSink:        liveSink,
		CandlePublisher: candlePub,
		Aggregator:      agg,
		Service:         svc,
	}, nil
}

// Run blocks until ctx is cancelled. Returns ctx.Err() on graceful
// shutdown or the first fatal error from the JetStream subscriber.
//
// On shutdown the CandlePublisher (if enabled) is drained AFTER
// Service.Run returns — the service handles tick-storage shutdown
// internally; module-level cleanup wraps around it.
func (m *Module) Run(ctx context.Context) error {
	err := m.Service.Run(ctx)
	if m.CandlePublisher != nil {
		m.CandlePublisher.Shutdown()
	}
	return err
}
