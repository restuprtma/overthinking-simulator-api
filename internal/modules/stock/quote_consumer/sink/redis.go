// Package sink writes the latest Quote state per stock into Redis as a
// hash. Updates are incremental — IQPlus Quote messages contain only the
// FIDs that changed, so each Redis write is an HSET (no overwrite of
// fields not present in the message).
//
// Storage layout:
//
//	HASH quote:<stock> {
//	    code:        "BBCA",
//	    prev_close:  "5400",
//	    last_price:  "5450",
//	    high_price:  "5475",
//	    ...                 // every named FID alias
//	    "60":        "5400" // raw FID number also stored for power users
//	    "56":        "5450"
//	    ...
//	    _updated_ts: "1714200000",
//	    _seq:        "12345",
//	}
package sink

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/quote_consumer/quote"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection params for the quote-state sink.
type Config struct {
	Addr        string // host:port
	Password    string
	DB          int
	KeyPrefix   string        // default "quote"
	StateTTL    time.Duration // default 25h — survives next pre-open
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultConfig returns sensible operational defaults.
func DefaultConfig() Config {
	return Config{
		KeyPrefix:   "quote",
		StateTTL:    25 * time.Hour,
		ClientName:  "quote-consumer",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// RedisSink writes incremental Quote updates to Redis.
type RedisSink struct {
	cli *redis.Client
	cfg Config
}

// New connects + pings Redis to fail fast on bad config.
func New(ctx context.Context, cfg Config) (*RedisSink, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "quote"
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

// Apply merges one Quote into Redis. Only the FIDs present in this Quote
// are touched — existing fields not in this update remain. Each FID is
// stored TWICE: once under its named alias (if known), once under its
// raw number. Consumer-side code can use whichever it prefers.
func (r *RedisSink) Apply(ctx context.Context, q quote.Quote, seq int64) error {
	if q.Code == "" || len(q.Fields) == 0 {
		return nil
	}

	key := r.cfg.KeyPrefix + ":" + q.Code

	// Pre-allocate: 2 entries per FID (raw + alias) + meta fields.
	args := make([]any, 0, len(q.Fields)*4+4)

	for fid, val := range q.Fields {
		args = append(args, fid, val)
		if alias, ok := quote.FidAlias[fid]; ok {
			args = append(args, alias, val)
		}
	}

	// Meta — always overwrite.
	args = append(args,
		"_updated_ts", strconv.FormatInt(time.Now().Unix(), 10),
		"_seq", strconv.FormatInt(seq, 10),
	)

	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, key, args...)
	pipe.Expire(ctx, key, r.cfg.StateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Shutdown closes the underlying Redis client.
func (r *RedisSink) Shutdown() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}
