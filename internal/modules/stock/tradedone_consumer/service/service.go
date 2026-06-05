package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/tradedone_consumer/parser"
	"tuai/internal/modules/stock/tradedone_consumer/sink"
	"tuai/internal/modules/stock/tradedone_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick time.Duration
}

// Service connects subscriber → parser → Redis sink. State is in Redis;
// no in-memory mutable state in this consumer.
type Service struct {
	sink        *sink.RedisSink
	cfg         Config
	subscriber  *subscriber.Subscriber
	parseErrors uint64
	dropped     uint64
}

func New(s *sink.RedisSink, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{sink: s, cfg: cfg}
}

// Handler returns the subscriber callback.
func (s *Service) Handler() subscriber.Handler {
	return s.onEnvelope
}

// AttachSubscriber wires the subscriber back-reference.
func (s *Service) AttachSubscriber(sub *subscriber.Subscriber) {
	s.subscriber = sub
}

// Run starts the subscriber and blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if s.subscriber == nil {
		panic("service.Run: subscriber not attached")
	}
	if err := s.subscriber.Start(ctx); err != nil {
		return err
	}
	defer s.shutdown()

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
		}
	}
}

func (s *Service) onEnvelope(ctx context.Context, env iqplus_envelope.Envelope) error {
	if env.RecordType != 40 {
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	td, err := parser.Parse(env.Data)
	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("tradedone parse error",
			zap.Int64("seq", env.Sequence),
			zap.String("data", env.Data),
			zap.Error(err))
		return nil
	}

	if err := s.sink.Apply(ctx, td, env.Sequence); err != nil {
		logger.Log.Warn("redis apply error",
			zap.String("stock", td.Code),
			zap.Float64("price", td.Price),
			zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) shutdown() {
	logger.Log.Info("tradedone consumer shutting down")
	s.subscriber.Close()
	if err := s.sink.Shutdown(); err != nil {
		logger.Log.Warn("redis shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("tradedone consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
