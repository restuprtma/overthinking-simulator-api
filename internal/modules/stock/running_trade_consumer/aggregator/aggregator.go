// Package aggregator turns a stream of Trade ticks into OHLCV bars per
// (stock, timeframe). Multiple timeframes are supported in parallel — one
// tick fans out into N bar updates, one per configured timeframe.
//
// Concurrency: Aggregator is single-goroutine. The subscriber dispatches
// trades sequentially, and OnTrade mutates state without locks.
package aggregator

import (
	"context"
	"time"

	"tuai/internal/modules/stock/running_trade_consumer/trade"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Bar is one OHLCV bucket for a single stock and timeframe.
type Bar struct {
	Stock     string
	Timeframe string    // "1m", "5m", ...
	OpenTime  time.Time // bucket start, UTC
	CloseTime time.Time // bucket end (exclusive), UTC
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Trades    int64 // tick count this bucket
}

// Sink is implemented by anything that consumes bar updates.
//
// Update is called on every tick (live current bar). Close is called when
// the bar's window ends and a new bucket begins. Implementations MUST be
// fast — they run on the subscriber dispatch goroutine.
type Sink interface {
	Update(ctx context.Context, b Bar) error
	Close(ctx context.Context, b Bar) error
}

// Timeframe pairs a Go duration with its short label. The label is what
// downstream code sees in Bar.Timeframe — used to build Redis keys
// (`candle:<stock>:<label>`) and NATS subjects (`idx.candle.<stock>.<label>`).
type Timeframe struct {
	Duration time.Duration
	Label    string
}

// Config holds aggregator knobs. Timeframes is the ordered list of buckets
// the aggregator maintains in parallel; each tick fans out to one update
// per timeframe.
type Config struct {
	Timeframes []Timeframe
}

// DefaultConfig produces a 1-minute-only aggregator (the historical default).
func DefaultConfig() Config {
	return Config{Timeframes: []Timeframe{{Duration: time.Minute, Label: "1m"}}}
}

// Aggregator buckets ticks per (stock, timeframe). State layout:
//
//	bars[stockCode][timeframeLabel] = current open bar
//
// One stock can have N concurrent bars, one per configured timeframe.
type Aggregator struct {
	cfg   Config
	sinks []Sink
	bars  map[string]map[string]*Bar
}

// New builds an Aggregator. If cfg.Timeframes is empty, falls back to the
// 1m default. Empty per-timeframe Label is filled in from Duration.String().
func New(cfg Config, sinks ...Sink) *Aggregator {
	if len(cfg.Timeframes) == 0 {
		cfg = DefaultConfig()
	}
	for i, tf := range cfg.Timeframes {
		if tf.Duration <= 0 {
			// Should never happen; loader validates. Belt-and-suspenders.
			cfg.Timeframes[i] = Timeframe{Duration: time.Minute, Label: "1m"}
			continue
		}
		if tf.Label == "" {
			cfg.Timeframes[i].Label = tf.Duration.String()
		}
	}
	return &Aggregator{
		cfg:   cfg,
		sinks: sinks,
		bars:  make(map[string]map[string]*Bar, 1024),
	}
}

// OnTrade routes one parsed Trade into its bucket and notifies all sinks.
//
// Withdrawn trades (Command=1) are ignored. Withdrawn means the trade was
// later cancelled by IDX — not part of OHLCV.
func (a *Aggregator) OnTrade(ctx context.Context, t trade.Trade) error {
	if !t.IsMatched() {
		return nil
	}
	if t.Code == "" || t.Volume <= 0 {
		// Defensive: malformed payloads shouldn't reach here, but if they
		// do, drop quietly rather than poison the bar.
		return nil
	}

	stockBars, ok := a.bars[t.Code]
	if !ok {
		stockBars = make(map[string]*Bar, len(a.cfg.Timeframes))
		a.bars[t.Code] = stockBars
	}

	for _, tf := range a.cfg.Timeframes {
		bucket := t.Timestamp.Truncate(tf.Duration)
		bar, exists := stockBars[tf.Label]

		if exists && bar.OpenTime.Equal(bucket) {
			// Same bucket → update in place.
			if t.Price > bar.High {
				bar.High = t.Price
			}
			if t.Price < bar.Low {
				bar.Low = t.Price
			}
			bar.Close = t.Price
			bar.Volume += t.Volume
			bar.Trades++
		} else {
			// New bucket for this timeframe. If we had a previous bar,
			// finalize it first.
			if exists {
				if err := a.fanoutClose(ctx, *bar); err != nil {
					logger.Log.Warn("close sink error",
						zap.String("stock", bar.Stock),
						zap.String("tf", bar.Timeframe),
						zap.Time("open_time", bar.OpenTime),
						zap.Error(err))
				}
			}
			bar = &Bar{
				Stock:     t.Code,
				Timeframe: tf.Label,
				OpenTime:  bucket,
				CloseTime: bucket.Add(tf.Duration),
				Open:      t.Price,
				High:      t.Price,
				Low:       t.Price,
				Close:     t.Price,
				Volume:    t.Volume,
				Trades:    1,
			}
			stockBars[tf.Label] = bar
		}

		if err := a.fanoutUpdate(ctx, *bar); err != nil {
			// First failing TF aborts the rest — subscriber NAK will
			// re-deliver the tick, all TFs replay from a consistent state.
			return err
		}
	}
	return nil
}

// FlushAll finalizes every open bar across all (stock, timeframe) — call
// before shutdown so the last in-flight buckets reach the closed-bar sinks.
func (a *Aggregator) FlushAll(ctx context.Context) {
	for code, stockBars := range a.bars {
		for tfLabel, bar := range stockBars {
			if err := a.fanoutClose(ctx, *bar); err != nil {
				logger.Log.Warn("flush close error",
					zap.String("stock", code),
					zap.String("tf", tfLabel),
					zap.Error(err))
			}
			delete(stockBars, tfLabel)
		}
		delete(a.bars, code)
	}
}

func (a *Aggregator) fanoutUpdate(ctx context.Context, b Bar) error {
	for _, s := range a.sinks {
		if err := s.Update(ctx, b); err != nil {
			// One sink failing should not block the others; surface as
			// the first error returned (subscriber will NAK the message).
			return err
		}
	}
	return nil
}

func (a *Aggregator) fanoutClose(ctx context.Context, b Bar) error {
	var firstErr error
	for _, s := range a.sinks {
		if err := s.Close(ctx, b); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
