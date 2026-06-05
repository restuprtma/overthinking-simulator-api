// Package sink implements destinations the OHLCV aggregator writes into.
//
// Redis sink: live bar state per (stock, timeframe). Key format:
// candle:<stock>:<timeframe>. Value is a hash with OHLCV fields.
//
// QuestDB sink: raw tick stream into the `running_trades` table — historical
// OHLCV is derived later via SAMPLE BY queries, not stored as a separate table.
package sink

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/running_trade_consumer/aggregator"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis connection params for the live-bar sink.
type RedisConfig struct {
	Addr        string // host:port, e.g. localhost:6379
	Password    string
	DB          int
	KeyPrefix   string        // default "candle"
	BarTTL      time.Duration // default 25h — survives next pre-open
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultRedisConfig returns sensible operational defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		KeyPrefix:   "candle",
		BarTTL:      25 * time.Hour,
		ClientName:  "running-trade-consumer",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// RedisSink writes the live-current bar to Redis on every tick.
type RedisSink struct {
	cli *redis.Client
	cfg RedisConfig
}

// NewRedis builds a RedisSink and pings the server to fail fast on bad
// config — call before Service.Run() so the publisher does not start
// consuming if the sink is broken.
func NewRedis(ctx context.Context, cfg RedisConfig) (*RedisSink, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "candle"
	}
	if cfg.BarTTL == 0 {
		cfg.BarTTL = 25 * time.Hour
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

// Update writes the current (still-open) bar — overwriting the previous
// snapshot. HSET is atomic per field; the bar belongs to one writer so no
// race risk.
func (r *RedisSink) Update(ctx context.Context, b aggregator.Bar) error {
	key := r.key(b.Stock, b.Timeframe)
	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, key,
		"open", b.Open,
		"high", b.High,
		"low", b.Low,
		"close", b.Close,
		"volume", b.Volume,
		"trades", b.Trades,
		"open_ts", b.OpenTime.Unix(),
		"close_ts", b.CloseTime.Unix(),
		"updated_ts", time.Now().Unix(),
		"status", "live",
	)
	pipe.Expire(ctx, key, r.cfg.BarTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Close marks the bar as closed in Redis (status flag flipped). Consumer
// dashboards can use this to know "this bar is final, no more updates".
//
// Historical durable storage of closed bars belongs in QuestDB — Redis
// is the *live* state.
func (r *RedisSink) Close(ctx context.Context, b aggregator.Bar) error {
	key := r.key(b.Stock, b.Timeframe)
	return r.cli.HSet(ctx, key,
		"status", "closed",
		"close", b.Close,
		"high", b.High,
		"low", b.Low,
		"volume", b.Volume,
		"trades", b.Trades,
		"closed_at_ts", strconv.FormatInt(time.Now().Unix(), 10),
	).Err()
}

func (r *RedisSink) key(stock, tf string) string {
	return r.cfg.KeyPrefix + ":" + stock + ":" + tf
}

// Shutdown closes the underlying Redis client.
func (r *RedisSink) Shutdown() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}
