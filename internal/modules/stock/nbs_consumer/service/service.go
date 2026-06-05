package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/nbs_consumer/parser"
	"tuai/internal/modules/stock/nbs_consumer/sink"
	"tuai/internal/modules/stock/nbs_consumer/subscriber"
	"tuai/internal/modules/stock/running_trade_consumer/trade"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick    time.Duration
	FlushOnTick  bool          // call Flush() on the sink every StatsTick
	FlushTimeout time.Duration // bound flush call duration
}

// Service connects subscriber → parser → QuestDB sink. Type 58 (stock-
// centric) and Type 59 (broker-centric) records each have their own
// destination table — the sink decides routing from parser.NBS.Source.
type Service struct {
	sink        sink.Writer
	cfg         Config
	subscriber  *subscriber.Subscriber
	parseErrors uint64
	dropped     uint64
	cType58     uint64
	cType59     uint64
}

// New constructs a Service. The caller owns the sink lifecycle.
func New(s sink.Writer, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	if cfg.FlushTimeout <= 0 {
		cfg.FlushTimeout = 10 * time.Second
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
			if s.cfg.FlushOnTick {
				s.flush(ctx)
			}
		}
	}
}

func (s *Service) onEnvelope(ctx context.Context, env iqplus_envelope.Envelope) error {
	var (
		n   parser.NBS
		err error
	)
	switch env.RecordType {
	case 58:
		n, err = parser.ParseStock(env.Data)
		atomic.AddUint64(&s.cType58, 1)
	case 59:
		n, err = parser.ParseBroker(env.Data)
		atomic.AddUint64(&s.cType59, 1)
	default:
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("nbs parse error",
			zap.Int("type", env.RecordType),
			zap.Int64("seq", env.Sequence),
			zap.String("data", env.Data),
			zap.Error(err))
		return nil
	}

	market := trade.DeriveMarket(n.Stock)

	if err := s.sink.Apply(ctx, n, market, env.Date, env.Time, env.ReceivedAt, env.Sequence); err != nil {
		logger.Log.Warn("questdb write error",
			zap.String("stock", n.Stock),
			zap.String("broker", n.Broker),
			zap.String("market", market),
			zap.Int("source", n.Source),
			zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) flush(ctx context.Context) {
	fctx, cancel := context.WithTimeout(ctx, s.cfg.FlushTimeout)
	defer cancel()
	if err := s.sink.Flush(fctx); err != nil {
		logger.Log.Warn("questdb flush error", zap.Error(err))
	}
}

func (s *Service) shutdown(ctx context.Context) {
	logger.Log.Info("nbs consumer shutting down")
	s.subscriber.Close()
	fctx, cancel := context.WithTimeout(context.Background(), s.cfg.FlushTimeout)
	defer cancel()
	if err := s.sink.Shutdown(fctx); err != nil {
		logger.Log.Warn("questdb shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("nbs consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Uint64("type58", atomic.LoadUint64(&s.cType58)),
		zap.Uint64("type59", atomic.LoadUint64(&s.cType59)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
