// Package candle is the v1 "candle" channel — OHLCV bars per (stock,
// timeframe), snapshot-on-subscribe from Redis db 11 + live updates from
// NATS subject `idx.candle.<stock>.<tf>`.
//
// Wire protocol (channel-specific args, embedded in the v1 command):
//
//	{"op":"subscribe",   "channel":"candle", "stocks":["BBCA"], "tfs":["1m","5m"]}
//	{"op":"unsubscribe", "channel":"candle", "stocks":["BBCA"], "tfs":["1m"]}
//	{"op":"resync",      "channel":"candle", "stocks":["BBCA"], "tfs":["1m"]}
//
// Outbound:
//
//	{"type":"snapshot", "channel":"candle", "stock":"BBCA", "tf":"1m", "bar":{...}}
//	{"type":"update",   "channel":"candle", "stock":"BBCA", "tf":"1m", "bar":{...}}
//
// `bar.status` distinguishes live (in-progress bucket) vs closed (final).
// No separate "closed" message type — clients route on bar.status.
package candle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "tuai/internal/modules/stock/ws_gateway/v1"
	"tuai/pkg/logger"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Config tunes the candle channel.
type Config struct {
	// SubjectPrefix must match running-trade-consumer's candle publisher
	// (default "idx.candle"). The channel subscribes to "<prefix>.>".
	SubjectPrefix string

	// MaxSubscriptionsPerClient caps (stock × tf) pairs per connection.
	MaxSubscriptionsPerClient int
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		SubjectPrefix:             "idx.candle",
		MaxSubscriptionsPerClient: 100, // generous — chart watchlists can be wide
	}
}

// Channel implements v1.Channel for OHLCV candle streams.
type Channel struct {
	cfg    Config
	nc     *nats.Conn
	reader *Reader

	mu sync.RWMutex
	// subscribers: subject (idx.candle.<stock>.<tf>) → set of clients.
	subscribers map[string]map[*v1.Client]bool
	// clientState: client → set of subjects subscribed. For cleanup on
	// disconnect + per-connection cap enforcement.
	clientState map[*v1.Client]map[string]bool

	natsSub *nats.Subscription

	statRecv   uint64
	statFanOut uint64
	statSlow   uint64
}

// New constructs the channel. Hub.Register(New(...)) → Start(ctx).
func New(nc *nats.Conn, reader *Reader, cfg Config) *Channel {
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "idx.candle"
	}
	if cfg.MaxSubscriptionsPerClient <= 0 {
		cfg.MaxSubscriptionsPerClient = 100
	}
	return &Channel{
		cfg:         cfg,
		nc:          nc,
		reader:      reader,
		subscribers: make(map[string]map[*v1.Client]bool),
		clientState: make(map[*v1.Client]map[string]bool),
	}
}

// Name implements v1.Channel.
func (ch *Channel) Name() string { return "candle" }

// Start subscribes to the wildcard candle subject.
func (ch *Channel) Start(_ context.Context) error {
	pattern := ch.cfg.SubjectPrefix + ".>"
	sub, err := ch.nc.Subscribe(pattern, ch.onNATS)
	if err != nil {
		return fmt.Errorf("nats subscribe %s: %w", pattern, err)
	}
	ch.natsSub = sub
	logger.Log.Info("candle channel subscribed", zap.String("pattern", pattern))
	return nil
}

// Stop unsubscribes from NATS.
func (ch *Channel) Stop() {
	if ch.natsSub != nil {
		_ = ch.natsSub.Unsubscribe()
	}
}

// subscribeArgs is the channel-specific arg shape.
type subscribeArgs struct {
	Stocks []string `json:"stocks"`
	TFs    []string `json:"tfs"`
}

// HandleSubscribe registers (stock × tf) pairs and pushes initial snapshot
// for each newly-added pair. Cross-product is taken: ["BBCA","BMRI"] ×
// ["1m","5m"] → 4 subjects.
func (ch *Channel) HandleSubscribe(c *v1.Client, raw json.RawMessage) error {
	var a subscribeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid subscribe args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)
	tfs := normalizeTFs(a.TFs)
	if len(stocks) == 0 || len(tfs) == 0 {
		return fmt.Errorf("subscribe requires non-empty stocks and tfs")
	}

	type pair struct{ stock, tf string }
	var newlyAdded []pair

	ch.mu.Lock()
	set, ok := ch.clientState[c]
	if !ok {
		set = make(map[string]bool)
		ch.clientState[c] = set
	}
	for _, s := range stocks {
		for _, tf := range tfs {
			subject := ch.cfg.SubjectPrefix + "." + s + "." + tf
			if set[subject] {
				continue
			}
			if len(set) >= ch.cfg.MaxSubscriptionsPerClient {
				ch.mu.Unlock()
				return fmt.Errorf("max %d candle subscriptions per connection",
					ch.cfg.MaxSubscriptionsPerClient)
			}
			set[subject] = true
			if _, ok := ch.subscribers[subject]; !ok {
				ch.subscribers[subject] = make(map[*v1.Client]bool)
			}
			ch.subscribers[subject][c] = true
			newlyAdded = append(newlyAdded, pair{s, tf})
		}
	}
	ch.mu.Unlock()

	for _, p := range newlyAdded {
		ch.sendSnapshot(c, p.stock, p.tf)
	}
	return nil
}

// HandleUnsubscribe removes (stock × tf) pairs.
func (ch *Channel) HandleUnsubscribe(c *v1.Client, raw json.RawMessage) error {
	var a subscribeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid unsubscribe args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)
	tfs := normalizeTFs(a.TFs)

	ch.mu.Lock()
	defer ch.mu.Unlock()
	set, ok := ch.clientState[c]
	if !ok {
		return nil
	}
	for _, s := range stocks {
		for _, tf := range tfs {
			subject := ch.cfg.SubjectPrefix + "." + s + "." + tf
			if !set[subject] {
				continue
			}
			delete(set, subject)
			if subs, ok := ch.subscribers[subject]; ok {
				delete(subs, c)
				if len(subs) == 0 {
					delete(ch.subscribers, subject)
				}
			}
		}
	}
	return nil
}

// HandleResync re-sends snapshots without changing subscription set.
func (ch *Channel) HandleResync(c *v1.Client, raw json.RawMessage) error {
	var a subscribeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid resync args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)
	tfs := normalizeTFs(a.TFs)

	type pair struct{ stock, tf string }
	var target []pair
	ch.mu.RLock()
	set := ch.clientState[c]
	for _, s := range stocks {
		for _, tf := range tfs {
			subject := ch.cfg.SubjectPrefix + "." + s + "." + tf
			if set[subject] {
				target = append(target, pair{s, tf})
			}
		}
	}
	ch.mu.RUnlock()

	for _, p := range target {
		ch.sendSnapshot(c, p.stock, p.tf)
	}
	return nil
}

// OnDisconnect cleans up subscriber maps when the client drops.
func (ch *Channel) OnDisconnect(c *v1.Client) {
	ch.mu.Lock()
	set, ok := ch.clientState[c]
	if !ok {
		ch.mu.Unlock()
		return
	}
	for subject := range set {
		if subs, ok := ch.subscribers[subject]; ok {
			delete(subs, c)
			if len(subs) == 0 {
				delete(ch.subscribers, subject)
			}
		}
	}
	delete(ch.clientState, c)
	ch.mu.Unlock()
}

// snapshotMessage is the per-subject snapshot wire envelope.
type snapshotMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Stock   string `json:"stock"`
	TF      string `json:"tf"`
	Bar     *Bar   `json:"bar"`
}

func (ch *Channel) sendSnapshot(c *v1.Client, stock, tf string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	bar, err := ch.reader.Get(ctx, stock, tf)
	if err != nil {
		logger.Log.Warn("candle snapshot read failed",
			zap.String("client", c.ID()),
			zap.String("stock", stock), zap.String("tf", tf), zap.Error(err))
		ch.sendError(c, v1.ErrSnapshotFail,
			"snapshot read failed for "+stock+"/"+tf)
		return
	}
	body, err := json.Marshal(snapshotMessage{
		Type:    v1.TypeSnapshot,
		Channel: "candle",
		Stock:   stock,
		TF:      tf,
		Bar:     bar, // may be nil — FE renders empty chart slot
	})
	if err != nil {
		return
	}
	if !c.Send(body) {
		atomic.AddUint64(&ch.statSlow, 1)
		logger.Log.Warn("candle send buffer full on snapshot",
			zap.String("client", c.ID()))
	}
}

// updateMessage is the per-bar live update wire envelope.
type updateMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Stock   string `json:"stock"`
	TF      string `json:"tf"`
	Bar     *Bar   `json:"bar"`
}

// onNATS is the wildcard callback. Decode bar, build the wire envelope,
// fan out to subscribers of (stock, tf).
func (ch *Channel) onNATS(msg *nats.Msg) {
	atomic.AddUint64(&ch.statRecv, 1)
	subject := msg.Subject

	stock, tf := parseSubject(subject, ch.cfg.SubjectPrefix)
	if stock == "" {
		return
	}

	var bar Bar
	if err := json.Unmarshal(msg.Data, &bar); err != nil {
		logger.Log.Warn("candle payload decode failed",
			zap.String("subject", subject), zap.Error(err))
		return
	}
	out, err := json.Marshal(updateMessage{
		Type:    v1.TypeUpdate,
		Channel: "candle",
		Stock:   stock,
		TF:      tf,
		Bar:     &bar,
	})
	if err != nil {
		return
	}

	ch.mu.RLock()
	set := ch.subscribers[subject]
	if len(set) == 0 {
		ch.mu.RUnlock()
		return
	}
	targets := make([]*v1.Client, 0, len(set))
	for c := range set {
		targets = append(targets, c)
	}
	ch.mu.RUnlock()

	atomic.AddUint64(&ch.statFanOut, uint64(len(targets)))
	for _, c := range targets {
		if !c.Send(out) {
			atomic.AddUint64(&ch.statSlow, 1)
		}
	}
}

func (ch *Channel) sendError(c *v1.Client, code, msg string) {
	body, err := json.Marshal(v1.ErrorMessage{
		Type:    v1.TypeError,
		Code:    code,
		Msg:     msg,
		Channel: "candle",
	})
	if err != nil {
		return
	}
	c.Send(body)
}

// parseSubject extracts (stock, tf) from "<prefix>.<stock>.<tf>". Returns
// empty strings if the subject doesn't match the expected shape (caller
// then drops the message).
func parseSubject(subject, prefix string) (stock, tf string) {
	if !strings.HasPrefix(subject, prefix+".") {
		return "", ""
	}
	rest := subject[len(prefix)+1:]
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// normalizeStocks uppercases + trims; drops empties + duplicates.
func normalizeStocks(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// normalizeTFs lowercases + trims (e.g. "1M" → "1m"); drops empties +
// duplicates.
func normalizeTFs(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
