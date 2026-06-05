// Package sink writes Trade Done volume-profile snapshots to Redis.
//
// Storage layout — one hash per stock; each price level is one HASH FIELD,
// the value is a JSON object with all volume/frequency counters:
//
//	HASH tradedone:<stock>
//	  field "5400" → {"bvol":12500,"svol":11000,"bfreq":50,"sfreq":42,
//	                  "bvol_f":3000,"bfreq_f":12,"svol_f":1500,"sfreq_f":8}
//	  field "5410" → {...}
//	  ...
//
//	HASH tradedone:<stock>:_meta
//	  { last_price, last_updated_ts, _seq, level_count }
//
// Vendor sends cumulative totals per (stock, price) — sink HSETs with the
// new value (overwrites). Missing fields stay untouched, so reading
// HGETALL returns the FULL volume profile for that stock.
//
// Front-end use: HGETALL tradedone:<stock> → render volume-by-price chart.
package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/tradedone_consumer/parser"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection params.
type Config struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string        // default "tradedone"
	StateTTL    time.Duration // default 25h
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		KeyPrefix:   "tradedone",
		StateTTL:    25 * time.Hour,
		ClientName:  "tradedone-consumer",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// volumeAtPrice is the JSON payload stored per price level.
//
// Field names kept short — they live in every Redis value, so size matters
// when a stock has 100+ price levels.
type volumeAtPrice struct {
	BVol   int64 `json:"bvol"`
	SVol   int64 `json:"svol"`
	BFreq  int64 `json:"bfreq"`
	SFreq  int64 `json:"sfreq"`
	BVolF  int64 `json:"bvol_f"`
	BFreqF int64 `json:"bfreq_f"`
	SVolF  int64 `json:"svol_f"`
	SFreqF int64 `json:"sfreq_f"`
}

// RedisSink writes volume-profile updates to Redis.
type RedisSink struct {
	cli *redis.Client
	cfg Config
}

// New connects + pings Redis to fail fast on bad config.
func New(ctx context.Context, cfg Config) (*RedisSink, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "tradedone"
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

// Apply overwrites the volume-at-price entry for one (stock, price) pair
// and bumps the meta fields atomically via pipeline.
func (r *RedisSink) Apply(ctx context.Context, td parser.TradeDone, seq int64) error {
	if td.Code == "" {
		return nil
	}

	body, err := json.Marshal(volumeAtPrice{
		BVol:   td.BVol,
		SVol:   td.SVol,
		BFreq:  td.BFreq,
		SFreq:  td.SFreq,
		BVolF:  td.BVolF,
		BFreqF: td.BFreqF,
		SVolF:  td.SVolF,
		SFreqF: td.SFreqF,
	})
	if err != nil {
		return fmt.Errorf("marshal vol@price: %w", err)
	}

	stockKey := r.cfg.KeyPrefix + ":" + td.Code
	metaKey := stockKey + ":_meta"
	priceField := strconv.FormatFloat(td.Price, 'f', -1, 64)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, stockKey, priceField, string(body))
	pipe.Expire(ctx, stockKey, r.cfg.StateTTL)

	pipe.HSet(ctx, metaKey,
		"last_price", priceField,
		"last_updated_ts", ts,
		"_seq", seq,
	)
	pipe.HIncrBy(ctx, metaKey, "updates", 1)
	pipe.Expire(ctx, metaKey, r.cfg.StateTTL)

	_, err = pipe.Exec(ctx)
	return err
}

// Shutdown closes the underlying Redis client.
func (r *RedisSink) Shutdown() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}
