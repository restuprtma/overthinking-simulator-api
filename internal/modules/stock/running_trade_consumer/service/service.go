package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/running_trade_consumer/aggregator"
	"tuai/internal/modules/stock/running_trade_consumer/sink"
	"tuai/internal/modules/stock/running_trade_consumer/subscriber"
	"tuai/internal/modules/stock/running_trade_consumer/trade"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs not owned by sub-components.
type Config struct {
	StatsTick time.Duration
}

// Service glues parser, aggregator, and sinks together.
//
// Per-message flow:
//
//	envelope → trade.Parse → questdb.Write (raw tick)
//	                       → aggregator.OnTrade → redis.Update (live bar)
//
// QuestDB write happens FIRST so durable history is recorded even if
// Redis fails. If QuestDB fails, the message is NAKed and JetStream
// redelivers — guarantees no historical-tick gap as long as the stream
// retention window holds.
//
// The Subscriber is attached after construction (via AttachSubscriber)
// to break the chicken-and-egg between handler and consumer.
type Service struct {
	tickWriter  sink.TickWriter
	aggregator  *aggregator.Aggregator
	cfg         Config
	subscriber  *subscriber.Subscriber // nil until AttachSubscriber()
	dropped     uint64                 // non-trade record types received unexpectedly
	parseErrors uint64
}

func New(
	tickWriter sink.TickWriter,
	agg *aggregator.Aggregator,
	cfg Config,
) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{
		tickWriter: tickWriter,
		aggregator: agg,
		cfg:        cfg,
	}
}

// Handler returns the subscriber-compatible callback for incoming
// envelopes. Pass to subscriber.New() during wiring.
func (s *Service) Handler() subscriber.Handler {
	return s.onEnvelope
}

// AttachSubscriber gives the Service a back-reference to the subscriber
// for stats logging and graceful shutdown. Must be called before Run().
func (s *Service) AttachSubscriber(sub *subscriber.Subscriber) {
	s.subscriber = sub
}

// Run starts the subscriber and blocks until ctx is cancelled. On
// shutdown: stops the consumer, flushes pending bars and the QuestDB
// buffer, then drains NATS.
func (s *Service) Run(ctx context.Context) error {
	if s.subscriber == nil {
		panic("service.Run: subscriber not attached")
	}
	if err := s.subscriber.Start(ctx); err != nil {
		return err
	}
	defer s.shutdown(ctx)

	ticker := time.NewTicker(s.cfg.StatsTick)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			s.logStats(time.Since(start))
			return ctx.Err()
		case <-ticker.C:
			s.logStats(time.Since(start))
			// Safety net: flush ILP buffer in case auto-flush thresholds
			// (rows/interval) haven't tripped — guarantees the latest tick
			// is durable within one stats interval, even at very low rate.
			if err := s.tickWriter.Flush(ctx); err != nil {
				logger.Log.Warn("periodic questdb flush error", zap.Error(err))
			}
		}
	}
}

// onEnvelope is the subscriber Handler. Returning an error → NAK.
func (s *Service) onEnvelope(ctx context.Context, env iqplus_envelope.Envelope) error {
	// Filter is server-side (idx.trade.>) but defend in depth.
	if env.RecordType != 15 && env.RecordType != 27 {
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	t, err := trade.Parse(env.Data)
	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("trade parse error",
			zap.Int("type", env.RecordType),
			zap.Int64("seq", env.Sequence),
			zap.String("data", env.Data),
			zap.Error(err))
		// Bad payload — ack to skip rather than retry forever.
		return nil
	}

	// Derive IDX board (RG/TN/NG) from stock code suffix — IDX convention:
	// codes ending in "NG"/"TN" are NG/TN board, otherwise RG.
	t.Market = trade.DeriveMarket(t.Code)

	if err := s.tickWriter.Write(ctx, t); err != nil {
		// Durable storage failed — NAK, JetStream will redeliver.
		logger.Log.Warn("questdb write error",
			zap.String("stock", t.Code), zap.Error(err))
		return err
	}

	if err := s.aggregator.OnTrade(ctx, t); err != nil {
		logger.Log.Warn("aggregator error",
			zap.String("stock", t.Code), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) shutdown(ctx context.Context) {
	logger.Log.Info("running-trade consumer shutting down")

	s.subscriber.Close()
	s.aggregator.FlushAll(ctx)

	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.tickWriter.Flush(flushCtx); err != nil {
		logger.Log.Warn("questdb final flush error", zap.Error(err))
	}
	if err := s.tickWriter.Shutdown(flushCtx); err != nil {
		logger.Log.Warn("questdb shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("running-trade consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
