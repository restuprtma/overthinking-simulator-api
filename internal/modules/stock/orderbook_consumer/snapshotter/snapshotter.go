// Package snapshotter is the background goroutine that drains dirty state
// from book.Engine, computes per-stock deltas vs. the last drained state,
// and pushes:
//
//   - full snapshots to Redis db 9 (cold-start source for ws-gateway)
//   - delta envelopes to NATS subject idx.orderbook.delta.<stock>
//     (live fanout for ws-gateway)
//
// Why debounce: order events arrive at thousands per second at peak. A
// 100ms tick groups many same-stock events into one Redis pipeline + one
// NATS delta — enough freshness for any human-facing depth display while
// keeping infra cost predictable.
//
// Delta semantics: each delta carries absolute new values per changed
// price level (`a:"u"` upsert) and price-only entries for vanished levels
// (`a:"r"` remove). Applying a delta twice yields the same state
// (idempotent) — safe under re-delivery.
package snapshotter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/orderbook_consumer/book"
	"tuai/internal/modules/stock/orderbook_consumer/sink"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config controls the snapshotter loop.
type Config struct {
	Interval     time.Duration // default 100ms
	FlushTimeout time.Duration // bound per-cycle write duration
	DefaultPhase string        // initial phase string before any session event
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		Interval:     100 * time.Millisecond,
		FlushTimeout: 5 * time.Second,
		DefaultPhase: "unknown",
	}
}

// Stats are surfaced for ops/logging.
type Stats struct {
	Cycles         uint64
	StocksFlushed  uint64
	DeltasEmitted  uint64
	RedisErrors    uint64
	NATSErrors     uint64
	LastBatchSize  uint64
	NoChangeCycles uint64
}

// Snapshotter pulls dirty state from book.Engine and pushes to Redis + NATS.
type Snapshotter struct {
	engine *book.Engine
	redis  *sink.RedisSink
	natsS  *sink.NATSSink
	cfg    Config

	// previous holds the LAST snapshot drained per stock so we can compute
	// delta envelopes (current vs previous). Mutated only by the
	// snapshotter goroutine — no locking required.
	previous map[string]book.DirtySnapshot

	// phase is the current trading-session phase, set by the session
	// listener and read on every cycle for meta serialization.
	phaseMu sync.RWMutex
	phase   string

	stats Stats
}

// New constructs a snapshotter. natsS may be nil if Redis-only operation is
// desired (development/testing); production must wire NATS.
func New(engine *book.Engine, redisS *sink.RedisSink, natsS *sink.NATSSink, cfg Config) *Snapshotter {
	if cfg.Interval <= 0 {
		cfg.Interval = 100 * time.Millisecond
	}
	if cfg.FlushTimeout <= 0 {
		cfg.FlushTimeout = 5 * time.Second
	}
	if cfg.DefaultPhase == "" {
		cfg.DefaultPhase = "unknown"
	}
	return &Snapshotter{
		engine:   engine,
		redis:    redisS,
		natsS:    natsS,
		cfg:      cfg,
		previous: make(map[string]book.DirtySnapshot, 1024),
		phase:    cfg.DefaultPhase,
	}
}

// SetPhase updates the trading phase string used in published meta. Called
// by the session listener (Type 57 handler).
func (s *Snapshotter) SetPhase(phase string) {
	s.phaseMu.Lock()
	s.phase = phase
	s.phaseMu.Unlock()
}

// Phase returns the current trading phase (read-only access for tests).
func (s *Snapshotter) Phase() string {
	s.phaseMu.RLock()
	defer s.phaseMu.RUnlock()
	return s.phase
}

// OnEngineReset clears the snapshotter's previous-state cache. Call AFTER
// engine.Reset() so the next cycle sends a fresh snapshot from clean state
// (no stale delta references). Optionally publishes a NATS reset envelope.
func (s *Snapshotter) OnEngineReset(ctx context.Context, reason string) {
	s.previous = make(map[string]book.DirtySnapshot, len(s.previous))
	if s.natsS != nil {
		if err := s.natsS.PublishReset(ctx, "", reason); err != nil {
			logger.Log.Warn("publish reset failed", zap.String("reason", reason), zap.Error(err))
		}
	}
}

// Run loops until ctx.Done. On exit, performs a final drain so any pending
// dirty state lands in Redis + NATS before shutdown.
func (s *Snapshotter) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flushOnce(context.Background()) // final drain w/ fresh ctx
			return
		case <-ticker.C:
			s.flushOnce(ctx)
		}
	}
}

func (s *Snapshotter) flushOnce(ctx context.Context) {
	atomic.AddUint64(&s.stats.Cycles, 1)

	dirty := s.engine.DrainDirty()
	if len(dirty) == 0 {
		atomic.AddUint64(&s.stats.NoChangeCycles, 1)
		atomic.StoreUint64(&s.stats.LastBatchSize, 0)
		return
	}
	atomic.StoreUint64(&s.stats.LastBatchSize, uint64(len(dirty)))

	flushCtx, cancel := context.WithTimeout(ctx, s.cfg.FlushTimeout)
	defer cancel()

	phase := s.Phase()

	for _, cur := range dirty {
		// 1) Persist current full state to Redis (overwrites bid + ask).
		if err := s.redis.WriteStock(flushCtx, cur, phase); err != nil {
			atomic.AddUint64(&s.stats.RedisErrors, 1)
			logger.Log.Debug("redis write failed",
				zap.String("stock", cur.Stock),
				zap.Uint64("seq", cur.Seq),
				zap.Error(err))
			// Even on Redis failure, still publish delta — ws-gateway may
			// already be receiving live state via its NATS subscription.
		} else {
			atomic.AddUint64(&s.stats.StocksFlushed, 1)
		}

		// 2) Publish delta vs previous snapshot to NATS for live fanout.
		if s.natsS != nil {
			prev := s.previous[cur.Stock] // zero-value if first seen
			payload := buildDelta(prev, cur)
			if hasChanges(payload) {
				if err := s.natsS.PublishDelta(flushCtx, payload); err != nil {
					atomic.AddUint64(&s.stats.NATSErrors, 1)
					logger.Log.Debug("nats delta publish failed",
						zap.String("stock", cur.Stock),
						zap.Uint64("seq", cur.Seq),
						zap.Error(err))
				} else {
					atomic.AddUint64(&s.stats.DeltasEmitted, 1)
				}
			}
		}

		// 3) Cache current as the new previous so next cycle's diff is
		//    against this state. Stored AFTER publish so a publish error
		//    doesn't cause us to "lose" the unpublished delta — next cycle
		//    will recompute against the older previous and pick it up.
		s.previous[cur.Stock] = cur
	}
}

// buildDelta computes the wire-format delta envelope from previous + current
// drained snapshots. Levels still in shares at this point; conversion to
// lots happens inline (÷ book.LotSize).
func buildDelta(prev, cur book.DirtySnapshot) sink.DeltaPayload {
	out := sink.DeltaPayload{
		Type:    "delta",
		Stock:   cur.Stock,
		Seq:     cur.Seq,
		PrevSeq: prev.Seq,
		TS:      time.Now().UnixMilli(),
	}
	out.Bids = diffSide(prev.Bid, cur.Bid)
	out.Asks = diffSide(prev.Ask, cur.Ask)
	out.Summary = buildSummary(cur)
	return out
}

func diffSide(prev, cur map[float64]book.LevelStats) []sink.DeltaLevel {
	var changes []sink.DeltaLevel

	// Upserts: levels in cur that differ from (or weren't in) prev.
	for price, lvl := range cur {
		old, existed := prev[price]
		if existed && old == lvl {
			continue
		}
		changes = append(changes, sink.DeltaLevel{
			Price:        price,
			Lot:          lvl.Lot / book.LotSize,
			Freq:         lvl.OrderCount,
			LotForeign:   lvl.LotForeign / book.LotSize,
			OrderForeign: lvl.OrderForeign,
			Action:       "u",
		})
	}
	// Removes: levels in prev that vanished in cur.
	for price := range prev {
		if _, stillThere := cur[price]; stillThere {
			continue
		}
		changes = append(changes, sink.DeltaLevel{
			Price:  price,
			Action: "r",
		})
	}
	return changes
}

func buildSummary(cur book.DirtySnapshot) *sink.Summary {
	s := &sink.Summary{}
	for price, lvl := range cur.Bid {
		lots := lvl.Lot / book.LotSize
		s.TotalBidLot += lots
		s.TotalBidFreq += lvl.OrderCount
		if s.TopBid == 0 || price > s.TopBid {
			s.TopBid = price
		}
	}
	for price, lvl := range cur.Ask {
		lots := lvl.Lot / book.LotSize
		s.TotalAskLot += lots
		s.TotalAskFreq += lvl.OrderCount
		if s.TopAsk == 0 || price < s.TopAsk {
			s.TopAsk = price
		}
	}
	return s
}

func hasChanges(p sink.DeltaPayload) bool {
	return len(p.Bids) > 0 || len(p.Asks) > 0
}

// Snapshot returns a copy of current counters.
func (s *Snapshotter) Snapshot() Stats {
	return Stats{
		Cycles:         atomic.LoadUint64(&s.stats.Cycles),
		StocksFlushed:  atomic.LoadUint64(&s.stats.StocksFlushed),
		DeltasEmitted:  atomic.LoadUint64(&s.stats.DeltasEmitted),
		RedisErrors:    atomic.LoadUint64(&s.stats.RedisErrors),
		NATSErrors:     atomic.LoadUint64(&s.stats.NATSErrors),
		LastBatchSize:  atomic.LoadUint64(&s.stats.LastBatchSize),
		NoChangeCycles: atomic.LoadUint64(&s.stats.NoChangeCycles),
	}
}
