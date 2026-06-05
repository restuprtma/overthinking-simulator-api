package service

import (
	"context"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/internal/modules/stock/running_trade_consumer/trade"
	"tuai/internal/modules/stock/orderbook_consumer/book"
	"tuai/internal/modules/stock/orderbook_consumer/parser"
	"tuai/internal/modules/stock/orderbook_consumer/snapshotter"
	"tuai/internal/modules/stock/orderbook_consumer/subscriber"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds service-level knobs.
type Config struct {
	StatsTick time.Duration
}

// Service dispatches incoming envelopes to the order book engine.
//
// Type 15 (Trade) and Type 16 (Order) are both required for accurate
// book reconstruction:
//   - Type 16 commands 0/1 add new orders to the book
//   - Type 16 commands 2/3 cancel an existing order
//   - Type 15 fills the buyer + seller orders by trade volume; if an
//     order's remaining lot drops to 0, it leaves the book
//
// The engine is mutated synchronously on the dispatch goroutine.
// A separate snapshotter drains dirty state to Redis on its own ticker.
type Service struct {
	engine      *book.Engine
	snapshotter *snapshotter.Snapshotter
	cfg         Config
	subscriber  *subscriber.Subscriber

	parseOrderErrors  uint64
	parseTradeErrors  uint64
	parseStatusErrors uint64
	cOrders           uint64
	cTrades           uint64
	cStatusEvents     uint64
	cResets           uint64
	dropped           uint64
}

func New(engine *book.Engine, snap *snapshotter.Snapshotter, cfg Config) *Service {
	if cfg.StatsTick <= 0 {
		cfg.StatsTick = 30 * time.Second
	}
	return &Service{engine: engine, snapshotter: snap, cfg: cfg}
}

// Handler returns the subscriber callback.
func (s *Service) Handler() subscriber.Handler {
	return s.onEnvelope
}

// AttachSubscriber wires the subscriber back-reference.
func (s *Service) AttachSubscriber(sub *subscriber.Subscriber) {
	s.subscriber = sub
}

// Run starts the subscriber + snapshotter and blocks until ctx cancelled.
func (s *Service) Run(ctx context.Context) error {
	if s.subscriber == nil {
		panic("service.Run: subscriber not attached")
	}
	if err := s.subscriber.Start(ctx); err != nil {
		return err
	}
	defer s.subscriber.Close()

	go s.snapshotter.Run(ctx)

	ticker := time.NewTicker(s.cfg.StatsTick)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			s.logStats(time.Since(start))
			logger.Log.Info("orderbook consumer shutting down")
			return ctx.Err()
		case <-ticker.C:
			s.logStats(time.Since(start))
		}
	}
}

func (s *Service) onEnvelope(ctx context.Context, env iqplus_envelope.Envelope) error {
	switch env.RecordType {
	case 16:
		o, err := parser.Parse(env.Data)
		if err != nil {
			atomic.AddUint64(&s.parseOrderErrors, 1)
			logger.Log.Warn("order parse error",
				zap.Int64("seq", env.Sequence),
				zap.String("data", env.Data),
				zap.Error(err))
			return nil
		}
		s.engine.OnOrder(o)
		atomic.AddUint64(&s.cOrders, 1)
		return nil

	case 15:
		t, err := trade.Parse(env.Data)
		if err != nil {
			atomic.AddUint64(&s.parseTradeErrors, 1)
			logger.Log.Warn("trade parse error",
				zap.Int64("seq", env.Sequence),
				zap.String("data", env.Data),
				zap.Error(err))
			return nil
		}
		// Withdrawn trades (Command=1) are NOT a real fill — skip.
		if !t.IsMatched() {
			return nil
		}
		s.engine.OnTrade(t.Code, t.BuyerOrderNo, t.SellerOrderNo, t.Volume)
		atomic.AddUint64(&s.cTrades, 1)
		return nil

	case 57:
		st, err := parser.ParseStatus(env.Data)
		if err != nil {
			atomic.AddUint64(&s.parseStatusErrors, 1)
			logger.Log.Warn("status parse error",
				zap.Int64("seq", env.Sequence),
				zap.String("data", env.Data),
				zap.Error(err))
			return nil
		}
		atomic.AddUint64(&s.cStatusEvents, 1)
		s.snapshotter.SetPhase(st.Phase)
		if st.IsReset {
			logger.Log.Info("session reset triggered",
				zap.String("status_code", st.Code),
				zap.String("description", st.Description))
			s.engine.Reset()
			s.snapshotter.OnEngineReset(ctx, "session_"+st.Code)
			atomic.AddUint64(&s.cResets, 1)
		} else {
			logger.Log.Info("trading phase change",
				zap.String("status_code", st.Code),
				zap.String("phase", st.Phase),
				zap.String("description", st.Description))
		}
		return nil

	default:
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}
}

func (s *Service) logStats(elapsed time.Duration) {
	subStats := s.subscriber.Snapshot()
	engStats := s.engine.Snapshot()
	snapStats := s.snapshotter.Snapshot()
	rate := float64(subStats.Received) / elapsed.Seconds()

	logger.Log.Info("orderbook consumer stats",
		zap.Uint64("received", subStats.Received),
		zap.Uint64("acked", subStats.Acked),
		zap.Uint64("naked", subStats.Naked),
		zap.Uint64("decode_err", subStats.Errors),
		zap.Uint64("orders", atomic.LoadUint64(&s.cOrders)),
		zap.Uint64("trades", atomic.LoadUint64(&s.cTrades)),
		zap.Uint64("status_events", atomic.LoadUint64(&s.cStatusEvents)),
		zap.Uint64("resets", atomic.LoadUint64(&s.cResets)),
		zap.String("phase", s.snapshotter.Phase()),
		zap.Uint64("order_parse_err", atomic.LoadUint64(&s.parseOrderErrors)),
		zap.Uint64("trade_parse_err", atomic.LoadUint64(&s.parseTradeErrors)),
		zap.Uint64("status_parse_err", atomic.LoadUint64(&s.parseStatusErrors)),
		zap.Uint64("dropped", atomic.LoadUint64(&s.dropped)),
		zap.Uint64("eng_added_bid", engStats.AddedBid),
		zap.Uint64("eng_added_ask", engStats.AddedAsk),
		zap.Uint64("eng_cancel_bid", engStats.CancelledBid),
		zap.Uint64("eng_cancel_ask", engStats.CancelledAsk),
		zap.Uint64("eng_matched", engStats.Matched),
		zap.Uint64("eng_unknown", engStats.UnknownOrder),
		zap.Int("open_orders", engStats.OpenOrders),
		zap.Int("active_stocks", engStats.ActiveStocks),
		zap.Uint64("snap_cycles", snapStats.Cycles),
		zap.Uint64("snap_stocks_flushed", snapStats.StocksFlushed),
		zap.Uint64("snap_deltas_emitted", snapStats.DeltasEmitted),
		zap.Uint64("snap_redis_errors", snapStats.RedisErrors),
		zap.Uint64("snap_nats_errors", snapStats.NATSErrors),
		zap.Uint64("snap_last_size", snapStats.LastBatchSize),
		zap.Uint64("snap_no_change_cycles", snapStats.NoChangeCycles),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed.Round(time.Second)))
}
