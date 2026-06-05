package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/running_trade_consumer/trade"
	"tuai/internal/modules/stock/orderbook_consumer/parser"
	orderSink "tuai/internal/modules/stock/resend_order_consumer/sink"
	"tuai/internal/modules/stock/running_order_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick    time.Duration
	FlushOnTick  bool          // call Flush() on the sink every StatsTick
	FlushTimeout time.Duration // bound flush call duration
}

// Service receives Type 16 (Order) envelopes, re-parses them, and writes
// every row to QuestDB (`running_orders`).
//
// Mirrors running-trade-consumer's running_trades path but for orders. During
// market hours Broker is masked as "--"; the broker-real archive lives
// in `orders` (resend-order-consumer) — this service is the live event
// log.
type Service struct {
	sink        orderSink.OrderWriter
	cfg         Config
	subscriber  *subscriber.Subscriber
	parseErrors uint64
	tsErrors    uint64
	dropped     uint64
	written     uint64
}

// New constructs a Service. The caller owns the sink lifecycle.
func New(s orderSink.OrderWriter, cfg Config) *Service {
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
	// Filter is server-side (idx.order.>) but defend in depth.
	if env.RecordType != 16 {
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}

	o, err := parser.Parse(env.Data)
	if err != nil {
		atomic.AddUint64(&s.parseErrors, 1)
		logger.Log.Warn("order parse error",
			zap.Int64("seq", env.Sequence),
			zap.String("data", env.Data),
			zap.Error(err))
		return nil
	}
	o.RawDate = env.Date

	ts, err := parser.ResolveTimestamp(env.Date, env.Time, o.Time)
	if err != nil {
		atomic.AddUint64(&s.tsErrors, 1)
		logger.Log.Warn("order timestamp error",
			zap.Int64("seq", env.Sequence),
			zap.String("env_date", env.Date),
			zap.String("env_time", env.Time),
			zap.String("order_time", o.Time),
			zap.Error(err))
		return nil
	}

	market := trade.DeriveMarket(o.Stock)

	if err := s.sink.Write(ctx, o, market, ts); err != nil {
		logger.Log.Warn("questdb write error",
			zap.String("stock", o.Stock),
			zap.Int64("order_no", o.OrderNumber),
			zap.Error(err))
		return err
	}
	atomic.AddUint64(&s.written, 1)
	return nil
}

func (s *Service) flush(ctx context.Context) {
	flushCtx, cancel := context.WithTimeout(ctx, s.cfg.FlushTimeout)
	defer cancel()
	if err := s.sink.Flush(flushCtx); err != nil {
		logger.Log.Warn("periodic questdb flush error", zap.Error(err))
	}
}

func (s *Service) shutdown(ctx context.Context) {
	logger.Log.Info("running-order-consumer shutting down")
	s.subscriber.Close()

	flushCtx, cancel := context.WithTimeout(context.Background(), s.cfg.FlushTimeout)
	defer cancel()
	if err := s.sink.Flush(flushCtx); err != nil {
		logger.Log.Warn("questdb final flush error", zap.Error(err))
	}
	if err := s.sink.Shutdown(flushCtx); err != nil {
		logger.Log.Warn("questdb shutdown error", zap.Error(err))
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()
	logger.Log.Info("running-order-consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("parse_err", atomic.LoadUint64(&s.parseErrors)),
		zap.Uint64("ts_err", atomic.LoadUint64(&s.tsErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Uint64("written", atomic.LoadUint64(&s.written)),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
