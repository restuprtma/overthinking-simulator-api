// Redis snapshot reader for the orderbook channel. Reads the per-stock
// state written by orderbook_consumer's sink/redis.go.
//
// Layout (see docs/orderbook/protocol.md §1):
//
//	HASH  orderbook:<stock>:bid     field=<price-str>  value=JSON {l,f,lf,ff}
//	HASH  orderbook:<stock>:ask     field=<price-str>  value=JSON {l,f,lf,ff}
//	HASH  orderbook:<stock>:_meta   seq, top_bid, top_ask, totals, phase, ...
//
// The bid/ask hashes are unsorted by Redis; this reader parses all entries
// and sorts in-memory (bid desc, ask asc) before applying the requested
// depth cap. IDX books are small (~5-30 levels/side), so allocation is
// bounded and sort is trivial.

package orderbook

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds connection params for the orderbook snapshot source.
type RedisConfig struct {
	Addr        string
	Password    string
	DB          int    // default 9 (must match orderbook-consumer)
	KeyPrefix   string // default "orderbook"
	ClientName  string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// DefaultRedisConfig returns operational defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		DB:          9,
		KeyPrefix:   "orderbook",
		ClientName:  "ws-gateway-orderbook",
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	}
}

// Reader fetches per-stock book state from Redis.
type Reader struct {
	cli *redis.Client
	cfg RedisConfig
}

// NewReader connects + pings Redis to fail fast on bad config.
func NewReader(ctx context.Context, cfg RedisConfig) (*Reader, error) {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "orderbook"
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
		return nil, fmt.Errorf("redis ping (orderbook): %w", err)
	}
	return &Reader{cli: cli, cfg: cfg}, nil
}

// Close releases the underlying connection pool.
func (r *Reader) Close() error {
	if r.cli != nil {
		return r.cli.Close()
	}
	return nil
}

// Level is one price-level entry. Short JSON field names match the wire
// format and the published delta from orderbook_consumer/sink/nats.go.
type Level struct {
	Price        float64 `json:"p"`
	Lot          int64   `json:"l,omitempty"`
	Freq         int64   `json:"f,omitempty"`
	LotForeign   int64   `json:"lf,omitempty"`
	OrderForeign int64   `json:"ff,omitempty"`
}

// Summary mirrors the meta totals.
type Summary struct {
	TopBid       float64 `json:"top_bid,omitempty"`
	TopAsk       float64 `json:"top_ask,omitempty"`
	TotalBidLot  int64   `json:"total_bid_lot"`
	TotalAskLot  int64   `json:"total_ask_lot"`
	TotalBidFreq int64   `json:"total_bid_freq"`
	TotalAskFreq int64   `json:"total_ask_freq"`
}

// Snapshot is the full per-stock state assembled for sending to a client.
type Snapshot struct {
	Type    string   `json:"type"`
	Channel string   `json:"channel"`
	Stock   string   `json:"stock"`
	Seq     uint64   `json:"seq"`
	TS      int64    `json:"ts"`
	Phase   string   `json:"phase,omitempty"`
	Bids    []Level  `json:"bids"`
	Asks    []Level  `json:"asks"`
	Summary *Summary `json:"summary,omitempty"`
}

// rawLevel mirrors the JSON shape written by
// orderbook_consumer/sink/redis.go for each price field.
type rawLevel struct {
	Lot          int64 `json:"l"`
	Freq         int64 `json:"f"`
	LotForeign   int64 `json:"lf"`
	OrderForeign int64 `json:"ff"`
}

// Get returns a fully-formed snapshot ready to send. Returns (nil, nil)
// when the stock has no state in Redis (never been touched by the
// orderbook-consumer in this run) — caller decides how to surface that,
// typically by sending an empty snapshot so the FE knows its subscribe
// was acknowledged.
func (r *Reader) Get(ctx context.Context, stock string, depth int) (*Snapshot, error) {
	if stock == "" {
		return nil, fmt.Errorf("orderbook: stock required")
	}
	if depth <= 0 {
		depth = 10
	}
	bidKey := r.cfg.KeyPrefix + ":" + stock + ":bid"
	askKey := r.cfg.KeyPrefix + ":" + stock + ":ask"
	metaKey := r.cfg.KeyPrefix + ":" + stock + ":_meta"

	pipe := r.cli.Pipeline()
	bidCmd := pipe.HGetAll(ctx, bidKey)
	askCmd := pipe.HGetAll(ctx, askKey)
	metaCmd := pipe.HGetAll(ctx, metaKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("orderbook snapshot pipeline: %w", err)
	}

	meta, err := metaCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("orderbook meta hgetall: %w", err)
	}
	if len(meta) == 0 {
		return nil, nil
	}

	bidRaw, err := bidCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("orderbook bid hgetall: %w", err)
	}
	askRaw, err := askCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("orderbook ask hgetall: %w", err)
	}

	bids := parseLevels(bidRaw, false) // desc — best bid first
	asks := parseLevels(askRaw, true)  // asc — best ask first
	if len(bids) > depth {
		bids = bids[:depth]
	}
	if len(asks) > depth {
		asks = asks[:depth]
	}

	seq, _ := strconv.ParseUint(meta["seq"], 10, 64)
	return &Snapshot{
		Type:    "snapshot",
		Channel: "orderbook",
		Stock:   stock,
		Seq:     seq,
		TS:      time.Now().UnixMilli(),
		Phase:   meta["phase"],
		Bids:    bids,
		Asks:    asks,
		Summary: parseSummary(meta),
	}, nil
}

// parseLevels decodes one side's HGETALL result into a depth-sorted slice.
// ascending=true gives ask order (cheapest first), false gives bid order
// (most expensive first). Malformed entries are skipped silently — caller
// monitoring catches persistent shortage via empty book on a stock that
// should be active.
func parseLevels(raw map[string]string, ascending bool) []Level {
	out := make([]Level, 0, len(raw))
	for priceStr, body := range raw {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			continue
		}
		var lvl rawLevel
		if err := json.Unmarshal([]byte(body), &lvl); err != nil {
			continue
		}
		out = append(out, Level{
			Price:        price,
			Lot:          lvl.Lot,
			Freq:         lvl.Freq,
			LotForeign:   lvl.LotForeign,
			OrderForeign: lvl.OrderForeign,
		})
	}
	if ascending {
		sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	}
	return out
}

func parseSummary(meta map[string]string) *Summary {
	s := &Summary{}
	s.TopBid, _ = strconv.ParseFloat(meta["top_bid"], 64)
	s.TopAsk, _ = strconv.ParseFloat(meta["top_ask"], 64)
	s.TotalBidLot = parseI64(meta["total_bid_lot"])
	s.TotalAskLot = parseI64(meta["total_ask_lot"])
	s.TotalBidFreq = parseI64(meta["total_bid_freq"])
	s.TotalAskFreq = parseI64(meta["total_ask_freq"])
	return s
}

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
