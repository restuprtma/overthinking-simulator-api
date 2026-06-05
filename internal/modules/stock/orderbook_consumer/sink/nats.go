// NATS sink publishes delta + reset events for ws-gateway fanout.
//
// Subject layout (see docs/orderbook/protocol.md §2):
//
//	idx.orderbook.delta.<stock>   per snapshotter cycle when book changed
//	idx.orderbook.reset.<stock>   on engine reset (session begin / midnight)
//
// Snapshots are NOT published via NATS — ws-gateway reads them from Redis
// db 9 on client subscribe (see sink/redis.go). Keeps NATS traffic bounded
// to actual change events. If a stream + replay use case emerges later, a
// periodic snapshot publisher can be layered on top of this sink.
package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSConfig holds connection params + subject prefix.
type NATSConfig struct {
	URL            string
	Token          string
	ClientName     string
	SubjectPrefix  string // default "idx.orderbook"
	ConnectTimeout time.Duration
	ReconnectWait  time.Duration
	MaxReconnects  int
	// PublishAsync controls whether Publish blocks. Default false (sync) —
	// snapshotter is already throttled at 100ms cadence so publish latency
	// is not on hot per-event path.
	PublishAsync bool
}

// DefaultNATSConfig returns operational defaults.
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{
		ClientName:     "orderbook-consumer",
		SubjectPrefix:  "idx.orderbook",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1,
	}
}

// NATSSink publishes delta + reset envelopes.
type NATSSink struct {
	cfg  NATSConfig
	nc   *nats.Conn
	pubs uint64
	errs uint64
}

// NewNATS dials NATS up-front (fail-fast on bad config).
func NewNATS(cfg NATSConfig) (*NATSSink, error) {
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "idx.orderbook"
	}
	if cfg.ClientName == "" {
		cfg.ClientName = "orderbook-consumer"
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 2 * time.Second
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = -1
	}

	opts := []nats.Option{
		nats.Name(cfg.ClientName),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.Timeout(cfg.ConnectTimeout),
	}
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}
	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return &NATSSink{cfg: cfg, nc: nc}, nil
}

// DeltaLevel is one entry in a delta payload's bids/asks array.
type DeltaLevel struct {
	Price        float64 `json:"p"`
	Lot          int64   `json:"l,omitempty"`
	Freq         int64   `json:"f,omitempty"`
	LotForeign   int64   `json:"lf,omitempty"`
	OrderForeign int64   `json:"ff,omitempty"`
	Action       string  `json:"a"` // "u" upsert, "r" remove
}

// DeltaPayload is the JSON shape published to idx.orderbook.delta.<stock>.
type DeltaPayload struct {
	Type    string       `json:"type"`     // "delta"
	Stock   string       `json:"stock"`
	Seq     uint64       `json:"seq"`
	PrevSeq uint64       `json:"prev_seq"`
	TS      int64        `json:"ts"` // ms epoch
	Bids    []DeltaLevel `json:"bids,omitempty"`
	Asks    []DeltaLevel `json:"asks,omitempty"`
	Summary *Summary     `json:"summary,omitempty"`
}

// Summary mirrors the meta totals — optional in delta, included whenever
// top-of-book or aggregate totals shift so the FE can re-render the TOTAL
// row without re-computing from levels.
type Summary struct {
	TopBid       float64 `json:"top_bid,omitempty"`
	TopAsk       float64 `json:"top_ask,omitempty"`
	TotalBidLot  int64   `json:"total_bid_lot"`
	TotalAskLot  int64   `json:"total_ask_lot"`
	TotalBidFreq int64   `json:"total_bid_freq"`
	TotalAskFreq int64   `json:"total_ask_freq"`
}

// ResetPayload is published to idx.orderbook.reset.<stock> (or .all when
// stock is empty, signalling a global reset).
type ResetPayload struct {
	Type   string `json:"type"`             // "reset"
	Stock  string `json:"stock,omitempty"`  // empty = global
	Reason string `json:"reason,omitempty"` // e.g. "session_begin"
	TS     int64  `json:"ts"`
}

// PublishDelta sends the delta envelope to idx.orderbook.delta.<stock>.
func (s *NATSSink) PublishDelta(_ context.Context, p DeltaPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal delta: %w", err)
	}
	subject := s.cfg.SubjectPrefix + ".delta." + p.Stock
	if err := s.publish(subject, body); err != nil {
		atomic.AddUint64(&s.errs, 1)
		return err
	}
	atomic.AddUint64(&s.pubs, 1)
	return nil
}

// PublishReset signals consumers to clear local state. Use stock="" for
// a global reset (every subscribed stock).
func (s *NATSSink) PublishReset(_ context.Context, stock, reason string) error {
	body, err := json.Marshal(ResetPayload{
		Type:   "reset",
		Stock:  stock,
		Reason: reason,
		TS:     time.Now().UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("marshal reset: %w", err)
	}
	subject := s.cfg.SubjectPrefix + ".reset."
	if stock == "" {
		subject += "all"
	} else {
		subject += stock
	}
	if err := s.publish(subject, body); err != nil {
		atomic.AddUint64(&s.errs, 1)
		return err
	}
	atomic.AddUint64(&s.pubs, 1)
	return nil
}

func (s *NATSSink) publish(subject string, body []byte) error {
	if s.cfg.PublishAsync {
		// Best-effort: fire-and-forget, no guarantee of delivery. Caller is
		// responsible for accepting brief drops on reconnect.
		return s.nc.Publish(subject, body)
	}
	if err := s.nc.Publish(subject, body); err != nil {
		return err
	}
	return s.nc.Flush()
}

// Stats returns counters for observability.
func (s *NATSSink) Stats() (publishes, errors uint64) {
	return atomic.LoadUint64(&s.pubs), atomic.LoadUint64(&s.errs)
}

// Shutdown drains the NATS connection.
func (s *NATSSink) Shutdown() {
	if s.nc != nil {
		_ = s.nc.Drain()
	}
}
