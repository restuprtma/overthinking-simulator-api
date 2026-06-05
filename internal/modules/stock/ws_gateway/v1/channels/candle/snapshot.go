// Redis snapshot reader for the candle channel. Reads current OHLCV bar
// state written by running_trade_consumer's Redis sink.
//
// Layout (key pattern matches running_trade_consumer/sink/redis.go):
//
//	HASH  candle:<stock>:<tf>   fields o/h/l/c/v/trades/open_ts/close_ts/
//	                            updated_ts/status

package candle

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds connection params.
type RedisConfig struct {
	Addr        string
	Password    string
	DB          int    // default 11 (matches running-trade-consumer)
	KeyPrefix   string // default "candle"
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultRedisConfig returns operational defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		DB:          11,
		KeyPrefix:   "candle",
		ClientName:  "ws-gateway-candle",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// Reader is a thin wrapper around the Redis client knowing the candle
// hash layout.
type Reader struct {
	cli *redis.Client
	cfg RedisConfig
}

// NewReader connects + pings.
func NewReader(ctx context.Context, cfg RedisConfig) (*Reader, error) {
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
		return nil, fmt.Errorf("redis ping (candle): %w", err)
	}
	return &Reader{cli: cli, cfg: cfg}, nil
}

// Close releases the connection pool.
func (r *Reader) Close() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}

// Bar matches the candle publisher's payload — copied here (vs imported
// from legacy domain package) so the v1 channel is self-contained.
type Bar struct {
	Stock   string  `json:"stock"`
	TF      string  `json:"tf"`
	OpenTS  int64   `json:"open_ts"`
	CloseTS int64   `json:"close_ts"`
	Open    float64 `json:"o"`
	High    float64 `json:"h"`
	Low     float64 `json:"l"`
	Close   float64 `json:"c"`
	Volume  int64   `json:"v"`
	Trades  int64   `json:"trades"`
	Status  string  `json:"status"` // "live" | "closed"
}

// Get returns the current bar for (stock, tf), or nil if Redis has no key
// (no trade activity yet in that bucket — not an error).
func (r *Reader) Get(ctx context.Context, stock, tf string) (*Bar, error) {
	key := r.cfg.KeyPrefix + ":" + stock + ":" + tf
	fields, err := r.cli.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall %s: %w", key, err)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return &Bar{
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
