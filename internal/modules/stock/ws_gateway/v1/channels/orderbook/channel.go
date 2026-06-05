// Package orderbook is the v1 "orderbook" channel — depth-of-book per
// stock, snapshot-on-subscribe (from Redis db 9) + live delta stream
// (from NATS subjects `idx.orderbook.delta.<stock>` and
// `idx.orderbook.reset.<stock|all>`).
//
// Wire protocol (channel-specific args, embedded in the v1 command):
//
//	{"op":"subscribe",   "channel":"orderbook", "stocks":["BBCA"], "depth":10}
//	{"op":"unsubscribe", "channel":"orderbook", "stocks":["BBCA"]}
//	{"op":"resync",      "channel":"orderbook", "stocks":["BBCA"]}
//
// Sequence + idempotency semantics — see docs/orderbook/protocol.md §3.
package orderbook

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

// Config tunes the orderbook channel.
type Config struct {
	// SubjectPrefix matches what orderbook-consumer's NATSSink publishes
	// to — defaults "idx.orderbook". The channel subscribes to
	// "<prefix>.delta.>" and "<prefix>.reset.>".
	SubjectPrefix string

	// MaxStocksPerClient caps how many simultaneous orderbook
	// subscriptions a single connection can hold (cheap DoS guard).
	MaxStocksPerClient int

	// MaxDepth caps the per-client requested level count. Requests above
	// this are silently clamped.
	MaxDepth int
}

// DefaultConfig returns operational defaults.
func DefaultConfig() Config {
	return Config{
		SubjectPrefix:      "idx.orderbook",
		MaxStocksPerClient: 30,
		MaxDepth:           30,
	}
}

// Channel implements v1.Channel for the orderbook feed.
type Channel struct {
	cfg    Config
	nc     *nats.Conn
	reader *Reader

	mu sync.RWMutex
	// subscribers: stock → set of clients wanting deltas for it. Built
	// from per-client state in clientState.
	subscribers map[string]map[*v1.Client]bool
	// clientState: client → set of subscribed stocks + per-client knobs
	// (depth cap). Cleaned up via OnDisconnect.
	clientState map[*v1.Client]*clientState

	deltaSub *nats.Subscription
	resetSub *nats.Subscription

	// Stats
	statRecvDelta uint64
	statRecvReset uint64
	statFanOut    uint64
	statSlowDrop  uint64
}

type clientState struct {
	stocks map[string]bool
	depth  int
}

// New constructs the channel. Hub.Register(New(...)) → Start(ctx).
func New(nc *nats.Conn, reader *Reader, cfg Config) *Channel {
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "idx.orderbook"
	}
	if cfg.MaxStocksPerClient <= 0 {
		cfg.MaxStocksPerClient = 30
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 30
	}
	return &Channel{
		cfg:         cfg,
		nc:          nc,
		reader:      reader,
		subscribers: make(map[string]map[*v1.Client]bool),
		clientState: make(map[*v1.Client]*clientState),
	}
}

// Name implements v1.Channel.
func (ch *Channel) Name() string { return "orderbook" }

// Start subscribes to NATS delta + reset subjects.
func (ch *Channel) Start(_ context.Context) error {
	deltaPattern := ch.cfg.SubjectPrefix + ".delta.>"
	resetPattern := ch.cfg.SubjectPrefix + ".reset.>"

	dsub, err := ch.nc.Subscribe(deltaPattern, ch.onDelta)
	if err != nil {
		return fmt.Errorf("nats subscribe %s: %w", deltaPattern, err)
	}
	rsub, err := ch.nc.Subscribe(resetPattern, ch.onReset)
	if err != nil {
		_ = dsub.Unsubscribe()
		return fmt.Errorf("nats subscribe %s: %w", resetPattern, err)
	}
	ch.deltaSub = dsub
	ch.resetSub = rsub
	logger.Log.Info("orderbook channel subscribed",
		zap.String("delta", deltaPattern), zap.String("reset", resetPattern))
	return nil
}

// Stop unsubscribes from NATS.
func (ch *Channel) Stop() {
	if ch.deltaSub != nil {
		_ = ch.deltaSub.Unsubscribe()
	}
	if ch.resetSub != nil {
		_ = ch.resetSub.Unsubscribe()
	}
}

// subscribeArgs is the channel-specific arg shape.
type subscribeArgs struct {
	Stocks []string `json:"stocks"`
	Depth  int      `json:"depth,omitempty"`
}

// HandleSubscribe parses args + registers the client + pushes initial
// snapshot per stock.
func (ch *Channel) HandleSubscribe(c *v1.Client, raw json.RawMessage) error {
	var a subscribeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid subscribe args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)
	if len(stocks) == 0 {
		return fmt.Errorf("subscribe requires non-empty stocks")
	}

	depth := a.Depth
	if depth <= 0 {
		depth = 10
	} else if depth > ch.cfg.MaxDepth {
		depth = ch.cfg.MaxDepth
	}

	var newlyAdded []string
	ch.mu.Lock()
	st, ok := ch.clientState[c]
	if !ok {
		st = &clientState{stocks: make(map[string]bool), depth: depth}
		ch.clientState[c] = st
	} else {
		st.depth = depth // last-write-wins for depth across re-subscribes
	}
	for _, s := range stocks {
		if st.stocks[s] {
			continue
		}
		if len(st.stocks) >= ch.cfg.MaxStocksPerClient {
			ch.mu.Unlock()
			return fmt.Errorf("max %d orderbook subscriptions per connection",
				ch.cfg.MaxStocksPerClient)
		}
		st.stocks[s] = true
		set, ok := ch.subscribers[s]
		if !ok {
			set = make(map[*v1.Client]bool)
			ch.subscribers[s] = set
		}
		set[c] = true
		newlyAdded = append(newlyAdded, s)
	}
	ch.mu.Unlock()

	for _, s := range newlyAdded {
		ch.sendSnapshot(c, s, depth)
	}
	return nil
}

type unsubArgs struct {
	Stocks []string `json:"stocks"`
}

// HandleUnsubscribe removes stocks from the client's subscription set.
func (ch *Channel) HandleUnsubscribe(c *v1.Client, raw json.RawMessage) error {
	var a unsubArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid unsubscribe args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)

	ch.mu.Lock()
	st, ok := ch.clientState[c]
	if !ok {
		ch.mu.Unlock()
		return nil
	}
	for _, s := range stocks {
		if !st.stocks[s] {
			continue
		}
		delete(st.stocks, s)
		if set, ok := ch.subscribers[s]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(ch.subscribers, s)
			}
		}
	}
	ch.mu.Unlock()
	return nil
}

// HandleResync re-sends snapshots for stocks the client is already
// subscribed to. Does not change the subscription set — purely replay.
func (ch *Channel) HandleResync(c *v1.Client, raw json.RawMessage) error {
	var a subscribeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("invalid resync args: %w", err)
	}
	stocks := normalizeStocks(a.Stocks)

	ch.mu.RLock()
	st, ok := ch.clientState[c]
	if !ok {
		ch.mu.RUnlock()
		return nil
	}
	depth := st.depth
	target := make([]string, 0, len(stocks))
	for _, s := range stocks {
		if st.stocks[s] {
			target = append(target, s)
		}
	}
	ch.mu.RUnlock()

	for _, s := range target {
		ch.sendSnapshot(c, s, depth)
	}
	return nil
}

// OnDisconnect removes the client from all subscriber indexes.
func (ch *Channel) OnDisconnect(c *v1.Client) {
	ch.mu.Lock()
	st, ok := ch.clientState[c]
	if !ok {
		ch.mu.Unlock()
		return
	}
	for s := range st.stocks {
		if set, ok := ch.subscribers[s]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(ch.subscribers, s)
			}
		}
	}
	delete(ch.clientState, c)
	ch.mu.Unlock()
}

// sendSnapshot reads current state from Redis and pushes to the client.
// Best-effort — Redis hiccups log but do not close the connection.
func (ch *Channel) sendSnapshot(c *v1.Client, stock string, depth int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	snap, err := ch.reader.Get(ctx, stock, depth)
	if err != nil {
		logger.Log.Warn("orderbook snapshot read failed",
			zap.String("client", c.ID()),
			zap.String("stock", stock), zap.Error(err))
		ch.sendError(c, v1.ErrSnapshotFail, "snapshot read failed for "+stock)
		return
	}
	if snap == nil {
		// No state yet — send an empty snapshot so FE knows the subscribe
		// was acked. Seq=0 signals "engine hasn't seen this stock yet".
		snap = &Snapshot{
			Type:    v1.TypeSnapshot,
			Channel: "orderbook",
			Stock:   stock,
			Seq:     0,
			TS:      time.Now().UnixMilli(),
			Phase:   "unknown",
			Bids:    []Level{},
			Asks:    []Level{},
		}
	}
	body, err := json.Marshal(snap)
	if err != nil {
		return
	}
	if !c.Send(body) {
		atomic.AddUint64(&ch.statSlowDrop, 1)
		logger.Log.Warn("orderbook send buffer full on snapshot",
			zap.String("client", c.ID()), zap.String("stock", stock))
	}
}

// onDelta is the NATS callback for idx.orderbook.delta.<stock>. We
// re-wrap the JSON to inject `channel` (the consumer publishes without it
// because it doesn't know the WS protocol) and fan out to subscribers.
func (ch *Channel) onDelta(msg *nats.Msg) {
	atomic.AddUint64(&ch.statRecvDelta, 1)
	stock := stripPrefix(msg.Subject, ch.cfg.SubjectPrefix+".delta.")
	if stock == "" {
		return
	}
	body, ok := withChannel(msg.Data, "orderbook")
	if !ok {
		return
	}
	ch.fanOut(stock, body)
}

// onReset is the NATS callback for idx.orderbook.reset.<stock|all>. The
// special "all" tail broadcasts to every client that has any orderbook
// subscription; a specific stock fans out only to its subscribers.
func (ch *Channel) onReset(msg *nats.Msg) {
	atomic.AddUint64(&ch.statRecvReset, 1)
	stock := stripPrefix(msg.Subject, ch.cfg.SubjectPrefix+".reset.")
	if stock == "" {
		return
	}
	body, ok := withChannel(msg.Data, "orderbook")
	if !ok {
		return
	}
	if stock == "all" {
		ch.broadcastAll(body)
		return
	}
	ch.fanOut(stock, body)
}

// fanOut pushes payload to every client subscribed to stock. Slow clients
// — those with full send buffer — are dropped (counter incremented).
func (ch *Channel) fanOut(stock string, payload []byte) {
	ch.mu.RLock()
	set := ch.subscribers[stock]
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
		if !c.Send(payload) {
			atomic.AddUint64(&ch.statSlowDrop, 1)
			logger.Log.Warn("orderbook slow client — dropping send",
				zap.String("client", c.ID()), zap.String("stock", stock))
		}
	}
}

// broadcastAll sends to every client that has any orderbook subscription.
// Used for global reset signals.
func (ch *Channel) broadcastAll(payload []byte) {
	ch.mu.RLock()
	targets := make([]*v1.Client, 0, len(ch.clientState))
	for c := range ch.clientState {
		targets = append(targets, c)
	}
	ch.mu.RUnlock()
	for _, c := range targets {
		if !c.Send(payload) {
			atomic.AddUint64(&ch.statSlowDrop, 1)
		}
	}
}

func (ch *Channel) sendError(c *v1.Client, code, msg string) {
	body, err := json.Marshal(v1.ErrorMessage{
		Type:    v1.TypeError,
		Code:    code,
		Msg:     msg,
		Channel: "orderbook",
	})
	if err != nil {
		return
	}
	c.Send(body)
}

// withChannel returns a JSON object body with `"channel":<name>` injected.
// Uses a streaming decode to avoid materializing a full map for large
// delta payloads. Returns (out, true) on success, (nil, false) on
// malformed input.
func withChannel(in []byte, name string) ([]byte, bool) {
	// Cheap: decode into a map, set channel, re-marshal. Delta payloads are
	// small (~hundreds of bytes typically); allocation cost is acceptable
	// per-event for the throughput we're targeting (~10–100 events/s/stock).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(in, &raw); err != nil {
		return nil, false
	}
	chName, _ := json.Marshal(name)
	raw["channel"] = chName
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	return out, true
}

// stripPrefix returns the trailing component after `prefix`, or "" if
// the subject doesn't have that prefix.
func stripPrefix(subject, prefix string) string {
	if !strings.HasPrefix(subject, prefix) {
		return ""
	}
	return subject[len(prefix):]
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
