// Package snapshot reads the current bar state for one (stock, timeframe)
// from Redis. Layout matches what running_trade_consumer/sink/redis.go writes:
// hash at `<prefix>:<stock>:<tf>` with fields o/h/l/c/v/trades/open_ts/
// close_ts/updated_ts/status.
//
// MUST stay in sync with the writer's key format. If the writer's prefix
// changes, update KeyPrefix in env.
package snapshot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/ws_gateway/domain"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection params.
type Config struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string        // default "candle"
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		KeyPrefix:   "candle",
		ClientName:  "ws-gateway",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// Reader is a thin wrapper around the Redis client that knows the
// candle-hash layout.
type Reader struct {
	cli *redis.Client
	cfg Config
}

// New connects to Redis and pings to fail-fast.
func New(ctx context.Context, cfg Config) (*Reader, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "candle"
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
	return &Reader{cli: cli, cfg: cfg}, nil
}

// Get returns the current bar for (stock, tf), or (nil, nil) if the key
// doesn't exist (no recent trade activity in that bucket).
func (r *Reader) Get(ctx context.Context, stock, tf string) (*domain.Bar, error) {
	key := r.cfg.KeyPrefix + ":" + stock + ":" + tf
	fields, err := r.cli.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall %s: %w", key, err)
	}
	if len(fields) == 0 {
		return nil, nil
	}

	return &domain.Bar{
		Stock:   stock,
		TF:      tf,
		OpenTS:  toI64(fields["open_ts"]),
		CloseTS: toI64(fields["close_ts"]),
		Open:    toF64(fields["open"]),
		High:    toF64(fields["high"]),
		Low:     toF64(fields["low"]),
		Close:   toF64(fields["close"]),
		Volume:  toI64(fields["volume"]),
		Trades:  toI64(fields["trades"]),
		Status:  fields["status"],
	}, nil
}

// Close releases the underlying Redis connection.
func (r *Reader) Close() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}

func toF64(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func toI64(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
