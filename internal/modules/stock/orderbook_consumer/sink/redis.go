// Package sink persists order-book snapshots to Redis db 9 as the
// cold-start source-of-truth. ws-gateway reads from these keys when a
// client subscribes; the REST snapshot endpoint also reads from here.
//
// Storage layout (see docs/orderbook/protocol.md §1):
//
//	HASH  orderbook:<stock>:bid     field=<price-str>  value=JSON {l,f,lf,ff}
//	HASH  orderbook:<stock>:ask     field=<price-str>  value=JSON {l,f,lf,ff}
//	HASH  orderbook:<stock>:_meta   fields seq, last_change_ts_ns, top_bid,
//	                                top_ask, bid_levels, ask_levels,
//	                                total_bid_lot, total_ask_lot,
//	                                total_bid_freq, total_ask_freq, phase
//
// Units: engine stores shares; this sink divides by book.LotSize (100) on
// serialize so wire-format values are in IDX lot units. Integer division
// is OK because all IDX RG-board volumes are multiples of LotSize.
//
// Atomicity: one Redis pipeline per stock — both sides + meta updated
// together. Snapshotter calls WriteStock per dirty stock.
package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/orderbook_consumer/book"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection params.
type Config struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string        // default "orderbook"
	StateTTL    time.Duration // default 25h (one trading day + slack)
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		KeyPrefix:   "orderbook",
		StateTTL:    25 * time.Hour,
		ClientName:  "orderbook-consumer",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// RedisSink writes book snapshots to Redis.
type RedisSink struct {
	cli *redis.Client
	cfg Config
}

// New connects + pings Redis to fail fast on bad config.
func New(ctx context.Context, cfg Config) (*RedisSink, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "orderbook"
	}
	if cfg.StateTTL == 0 {
		cfg.StateTTL = 25 * time.Hour
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 2 * time.Second
	}

	cli := redis.NewClient(&redis.Options{
		Addr:        cfg.Addr,
		Password:    cfg.Password,
		DB:          cfg.DB,
		ClientName:  cfg.ClientName,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
	})

	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisSink{cli: cli, cfg: cfg}, nil
}

// wireLevel is the public JSON shape per price level.
type wireLevel struct {
	Lot          int64 `json:"l"`
	Freq         int64 `json:"f"`
	LotForeign   int64 `json:"lf"`
	OrderForeign int64 `json:"ff"`
}

// WriteStock atomically replaces both sides for one stock and updates the
// meta hash with the snapshot's seq and summary fields. One Redis pipeline
// per stock.
//
// Pipeline (per side):
//  1. DEL  <side> hash
//  2. HSET <side> hash with one field per surviving level (lot in lots)
//  3. EXPIRE <side>
// Plus meta:
//  4. HSET meta { seq, last_change_ts_ns, top_*, *_levels, total_*, phase }
//  5. EXPIRE meta
func (r *RedisSink) WriteStock(ctx context.Context, d book.DirtySnapshot, phase string) error {
	if d.Stock == "" {
		return nil
	}
	bidKey := r.cfg.KeyPrefix + ":" + d.Stock + ":bid"
	askKey := r.cfg.KeyPrefix + ":" + d.Stock + ":ask"
	metaKey := r.cfg.KeyPrefix + ":" + d.Stock + ":_meta"

	pipe := r.cli.Pipeline()
	pipe.Del(ctx, bidKey)
	pipe.Del(ctx, askKey)

	topBid, totalBidLot, totalBidFreq := writeSide(ctx, pipe, bidKey, d.Bid, book.SideBid)
	topAsk, totalAskLot, totalAskFreq := writeSide(ctx, pipe, askKey, d.Ask, book.SideAsk)

	pipe.Expire(ctx, bidKey, r.cfg.StateTTL)
	pipe.Expire(ctx, askKey, r.cfg.StateTTL)

	nowNs := time.Now().UnixNano()
	metaFields := []any{
		"seq", strconv.FormatUint(d.Seq, 10),
		"last_change_ts_ns", strconv.FormatInt(nowNs, 10),
		"bid_levels", strconv.Itoa(len(d.Bid)),
		"ask_levels", strconv.Itoa(len(d.Ask)),
		"total_bid_lot", strconv.FormatInt(totalBidLot, 10),
		"total_ask_lot", strconv.FormatInt(totalAskLot, 10),
		"total_bid_freq", strconv.FormatInt(totalBidFreq, 10),
		"total_ask_freq", strconv.FormatInt(totalAskFreq, 10),
		"phase", phase,
	}
	if topBid > 0 {
		metaFields = append(metaFields, "top_bid", strconv.FormatFloat(topBid, 'f', -1, 64))
	} else {
		pipe.HDel(ctx, metaKey, "top_bid")
	}
	if topAsk > 0 {
		metaFields = append(metaFields, "top_ask", strconv.FormatFloat(topAsk, 'f', -1, 64))
	} else {
		pipe.HDel(ctx, metaKey, "top_ask")
	}
	pipe.HSet(ctx, metaKey, metaFields...)
	pipe.Expire(ctx, metaKey, r.cfg.StateTTL)

	_, err := pipe.Exec(ctx)
	return err
}

// writeSide pushes one side's levels into the pipeline and returns
// (best_price, total_lot, total_freq) — best_price is the top-of-book for
// the side (max for bid, min for ask), 0 if no levels.
//
// Internal state is in shares; serialized value is in lots (÷ LotSize).
func writeSide(
	ctx context.Context,
	pipe redis.Pipeliner,
	key string,
	levels map[float64]book.LevelStats,
	side book.Side,
) (best float64, totalLot, totalFreq int64) {
	for price, lvl := range levels {
		if lvl.Lot <= 0 {
			continue
		}
		body, err := json.Marshal(wireLevel{
			Lot:          lvl.Lot / book.LotSize,
			Freq:         lvl.OrderCount,
			LotForeign:   lvl.LotForeign / book.LotSize,
			OrderForeign: lvl.OrderForeign,
		})
		if err != nil {
			continue
		}
		pipe.HSet(ctx, key, strconv.FormatFloat(price, 'f', -1, 64), string(body))

		totalLot += lvl.Lot / book.LotSize
		totalFreq += lvl.OrderCount
		if best == 0 {
			best = price
		} else if side == book.SideBid && price > best {
			best = price
		} else if side == book.SideAsk && price < best {
			best = price
		}
	}
	return
}

// Shutdown closes the underlying Redis client.
func (r *RedisSink) Shutdown() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}
