package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/quote_consumer/quote"
	"tuai/internal/modules/stock/quote_consumer/sink"
	"tuai/internal/modules/stock/quote_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick time.Duration
}

// Service connects subscriber → quote parser → Redis sink. It does not
// own any heavy state — the Redis hash IS the state.
type Service struct {
	sink        *sink.RedisSink
	cfg         Config
	subscriber  *subscriber.Subscriber // attached after construction
	parseErrors uint64
	dropped     uint64 // non-quote types received unexpectedly
}

func New(s *sink.RedisSink, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{sink: s, cfg: cfg}
}

// Handler returns the subscriber-compatible callback. Pass to
// subscriber.New() during wiring.
func (s *Service) Handler() subscriber.Handler {
	return s.onEnvelope
}

// AttachSubscriber gives the Service a back-reference for stats logging
// and shutdown coordination. Must be called before Run().
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
	// Server-side filter is idx.quote.> (record type 14). Defend in depth.
	if env.RecordType != 14 {
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	q, err := quote.Parse(env.Data)
	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("quote parse error",
			zap.Int64("seq", env.Sequence),
			zap.String("data", env.Data),
			zap.Error(err))
		// Bad payload — ack to skip rather than retry forever.
		return nil
	}

	if err := s.sink.Apply(ctx, q, env.Sequence); err != nil {
		logger.Log.Warn("redis apply error",
			zap.String("stock", q.Code), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) shutdown() {
	logger.Log.Info("quote consumer shutting down")
	s.subscriber.Close()
	if err := s.sink.Shutdown(); err != nil {
		logger.Log.Warn("redis shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("quote consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
