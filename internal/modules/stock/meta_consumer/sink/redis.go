// Package sink writes the latest market metadata into Redis.
//
// Storage layout:
//
//	HASH market:control                       { state, updated_ts }
//	HASH market:session                       { code, label, description, updated_ts }
//	LIST market:session:history (max 100)     "<ts>|<code>|<label>" entries (LPUSH + LTRIM)
//	HASH market:activity                      { inactive, active, down, nochg, up, updated_ts }
//	HASH market:summary:<stype>:<board>       { frequency, volume, value, ... }
//	LIST market:top20:<type_code>             20 codes (RPUSH after DEL)
//	LIST market:top20:<type_label>            same codes, friendly key
//
// All keys carry a TTL (default 25h) so stale state evicts naturally if
// the consumer stops but the Redis instance keeps running.
package sink

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"tuai/internal/modules/stock/meta_consumer/parser"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection params.
type Config struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string
	StateTTL    time.Duration
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		KeyPrefix:   "market",
		StateTTL:    25 * time.Hour,
		ClientName:  "meta-consumer",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// RedisSink writes meta-state to Redis.
type RedisSink struct {
	cli *redis.Client
	cfg Config
}

// New connects + pings Redis to fail fast on bad config.
func New(ctx context.Context, cfg Config) (*RedisSink, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "market"
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

// ------------------------------------------------------------------ helpers

func (r *RedisSink) key(parts ...string) string {
	out := r.cfg.KeyPrefix
	for _, p := range parts {
		out += ":" + p
	}
	return out
}

func nowSec() string { return strconv.FormatInt(time.Now().Unix(), 10) }

// ------------------------------------------------------------------ writers

// WriteControl stores feed UP/DOWN state.
func (r *RedisSink) WriteControl(ctx context.Context, c parser.Control, seq int64) error {
	k := r.key("control")
	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, k,
		"state", c.State,
		"updated_ts", nowSec(),
		"_seq", seq,
	)
	pipe.Expire(ctx, k, r.cfg.StateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// WriteStatus stores current trading session phase + appends to history.
func (r *RedisSink) WriteStatus(ctx context.Context, s parser.Status, seq int64) error {
	current := r.key("session")
	history := r.key("session:history")
	ts := nowSec()

	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, current,
		"code", s.Code,
		"label", s.Label,
		"description", s.Description,
		"updated_ts", ts,
		"_seq", seq,
	)
	pipe.Expire(ctx, current, r.cfg.StateTTL)

	entry := ts + "|" + s.Code + "|" + s.Label + "|" + s.Description
	pipe.LPush(ctx, history, entry)
	pipe.LTrim(ctx, history, 0, 99) // keep latest 100 transitions
	pipe.Expire(ctx, history, r.cfg.StateTTL)

	_, err := pipe.Exec(ctx)
	return err
}

// WriteActivity stores market-wide stock counters.
func (r *RedisSink) WriteActivity(ctx context.Context, a parser.Activity, seq int64) error {
	k := r.key("activity")
	pipe := r.cli.Pipeline()
	pipe.HSet(ctx, k,
		"inactive", a.Inactive,
		"active", a.Active,
		"down", a.Down,
		"nochg", a.NoChange,
		"up", a.Up,
		"updated_ts", nowSec(),
		"_seq", seq,
	)
	pipe.Expire(ctx, k, r.cfg.StateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// WriteSummary stores per-board trading summary. Key is stored under both
// raw stype code AND the friendly label, when known.
func (r *RedisSink) WriteSummary(ctx context.Context, s parser.Summary, seq int64) error {
	if s.Stype == "" || s.Board == "" {
		return nil
	}
	pipe := r.cli.Pipeline()
	for _, stype := range r.summaryAliases(s) {
		k := r.key("summary", stype, s.Board)
		pipe.HSet(ctx, k,
			"frequency", s.Frequency,
			"volume", s.Volume,
			"value", s.Value,
			"f_bought_freq", s.FBoughtFreq,
			"f_bought_vol", s.FBoughtVol,
			"f_bought_val", s.FBoughtVal,
			"f_sold_freq", s.FSoldFreq,
			"f_sold_vol", s.FSoldVol,
			"f_sold_val", s.FSoldVal,
			"updated_ts", nowSec(),
			"_seq", seq,
		)
		pipe.Expire(ctx, k, r.cfg.StateTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// summaryAliases returns the keys to write summary under: always raw,
// plus the label form when known.
func (r *RedisSink) summaryAliases(s parser.Summary) []string {
	out := []string{s.Stype}
	if s.StypeLabel != "" && s.StypeLabel != s.Stype {
		out = append(out, s.StypeLabel)
	}
	return out
}

// WriteTop20 replaces the snapshot list under both raw type code AND
// friendly label key. Atomic via pipeline DEL+RPUSH.
func (r *RedisSink) WriteTop20(ctx context.Context, t parser.Top20, seq int64) error {
	if t.TypeCode == "" || len(t.Codes) == 0 {
		return nil
	}
	keys := []string{r.key("top20", t.TypeCode)}
	if t.TypeLabel != "" && t.TypeLabel != t.TypeCode {
		keys = append(keys, r.key("top20", t.TypeLabel))
	}

	codes := make([]any, len(t.Codes))
	for i, c := range t.Codes {
		codes[i] = c
	}

	pipe := r.cli.Pipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
		pipe.RPush(ctx, k, codes...)
		pipe.Expire(ctx, k, r.cfg.StateTTL)
	}
	// Also write a small meta hash so consumers can introspect updated_ts
	// without LRANGE-ing the list itself.
	metaKey := r.key("top20", t.TypeCode, "_meta")
	pipe.HSet(ctx, metaKey,
		"label", t.TypeLabel,
		"size", len(t.Codes),
		"updated_ts", nowSec(),
		"_seq", seq,
	)
	pipe.Expire(ctx, metaKey, r.cfg.StateTTL)

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
