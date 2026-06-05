package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/meta_consumer/parser"
	"tuai/internal/modules/stock/meta_consumer/sink"
	"tuai/internal/modules/stock/meta_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick time.Duration
}

// Service dispatches each envelope to the correct parser + sink writer
// based on RecordType. All five record types land in a single Redis
// instance — no shared mutable state, so dispatch is straight-line.
type Service struct {
	sink       *sink.RedisSink
	cfg        Config
	subscriber *subscriber.Subscriber

	// per-type counters for visibility
	cControl  uint64
	cStatus   uint64
	cActivity uint64
	cSummary  uint64
	cTop20    uint64
	cDropped  uint64
	cParseErr uint64
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

// AttachSubscriber wires the subscriber back-reference for stats + shutdown.
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
	switch env.RecordType {
	case 13: // Control
		ev, err := parser.ParseControl(env.Data)
		if err != nil {
			atomic.AddUint64(&s.cParseErr, 1)
			s.logParseErr("control", env, err)
			return nil // bad payload, ack
		}
		atomic.AddUint64(&s.cControl, 1)
		return s.sink.WriteControl(ctx, ev, env.Sequence)

	case 17: // Top 20
		ev, err := parser.ParseTop20(env.Data)
		if err != nil {
			atomic.AddUint64(&s.cParseErr, 1)
			s.logParseErr("top20", env, err)
			return nil
		}
		atomic.AddUint64(&s.cTop20, 1)
		return s.sink.WriteTop20(ctx, ev, env.Sequence)

	case 39: // Stock Activity
		ev, err := parser.ParseActivity(env.Data)
		if err != nil {
			atomic.AddUint64(&s.cParseErr, 1)
			s.logParseErr("activity", env, err)
			return nil
		}
		atomic.AddUint64(&s.cActivity, 1)
		return s.sink.WriteActivity(ctx, ev, env.Sequence)

	case 57: // Trading Status
		ev, err := parser.ParseStatus(env.Data)
		if err != nil {
			atomic.AddUint64(&s.cParseErr, 1)
			s.logParseErr("status", env, err)
			return nil
		}
		atomic.AddUint64(&s.cStatus, 1)
		return s.sink.WriteStatus(ctx, ev, env.Sequence)

	case 130: // Trading Summary
		ev, err := parser.ParseSummary(env.Data)
		if err != nil {
			atomic.AddUint64(&s.cParseErr, 1)
			s.logParseErr("summary", env, err)
			return nil
		}
		atomic.AddUint64(&s.cSummary, 1)
		return s.sink.WriteSummary(ctx, ev, env.Sequence)

	default:
		atomic.AddUint64(&s.cDropped, 1)
		return nil
	}
}

func (s *Service) logParseErr(kind string, env iqplus_envelope.Envelope, err error) {
	logger.Log.Warn("meta parse error",
		zap.String("kind", kind),
		zap.Int64("seq", env.Sequence),
		zap.String("data", env.Data),
		zap.Error(err))
}

func (s *Service) shutdown() {
	logger.Log.Info("meta consumer shutting down")
	s.subscriber.Close()
	if err := s.sink.Shutdown(); err != nil {
		logger.Log.Warn("redis shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("meta consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.cParseErr)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.cDropped)),
		zap.Uint64("control", atomic.LoadUint64(&s.cControl)),
		zap.Uint64("status", atomic.LoadUint64(&s.cStatus)),
		zap.Uint64("activity", atomic.LoadUint64(&s.cActivity)),
		zap.Uint64("summary", atomic.LoadUint64(&s.cSummary)),
		zap.Uint64("top20", atomic.LoadUint64(&s.cTop20)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
