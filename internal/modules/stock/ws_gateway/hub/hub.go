package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"tuai/internal/modules/stock/ws_gateway/domain"
	"tuai/internal/modules/stock/ws_gateway/snapshot"
	"tuai/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Config holds runtime knobs for the Hub.
//
// SubjectPrefix MUST match what running-trade-consumer's candle publisher uses
// (default "idx.candle"). The hub subscribes once to "<prefix>.>".
type Config struct {
	SubjectPrefix string // default "idx.candle"
}

// Hub is the central fan-out manager. Single instance per ws-gateway
// process. Holds:
//
//   - one wildcard NATS subscription that catches every candle update
//   - the set of connected clients
//   - the subject → clients index used to fan out NATS messages
//
// The Hub is goroutine-safe via a single RWMutex; mutations (register /
// unregister / subscribe / unsubscribe) take the write lock, the NATS
// dispatch path takes the read lock.
type Hub struct {
	cfg Config

	mu       sync.RWMutex
	clients  map[*Client]bool
	wantedBy map[string]map[*Client]bool // subject → clients

	nc      *nats.Conn
	natsSub *nats.Subscription
	reader  *snapshot.Reader

	clientSeq atomic.Uint64
}

// New builds a Hub but does NOT subscribe to NATS — call Start() for that
// after the gateway HTTP server is ready to serve traffic.
func New(cfg Config, nc *nats.Conn, reader *snapshot.Reader) *Hub {
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "idx.candle"
	}
	return &Hub{
		cfg:      cfg,
		clients:  make(map[*Client]bool),
		wantedBy: make(map[string]map[*Client]bool),
		nc:       nc,
		reader:   reader,
	}
}

// Start subscribes to the wildcard candle subject. Returns an error if
// the subscription cannot be established.
func (h *Hub) Start(_ context.Context) error {
	pattern := h.cfg.SubjectPrefix + ".>"
	sub, err := h.nc.Subscribe(pattern, h.onNATS)
	if err != nil {
		return fmt.Errorf("nats subscribe %s: %w", pattern, err)
	}
	h.natsSub = sub
	logger.Log.Info("ws-gateway hub started",
		zap.String("subject_pattern", pattern))
	return nil
}

// Stop unsubscribes from NATS and closes every connected client.
func (h *Hub) Stop() {
	if h.natsSub != nil {
		_ = h.natsSub.Unsubscribe()
	}
	h.mu.Lock()
	for c := range h.clients {
		c.closeSend()
	}
	h.clients = map[*Client]bool{}
	h.wantedBy = map[string]map[*Client]bool{}
	h.mu.Unlock()
}

// Register accepts a freshly-upgraded WebSocket connection, wraps it in a
// Client, and starts the read/write pumps. Blocking — caller is the HTTP
// handler goroutine and we want the connection to stay alive there.
func (h *Hub) Register(conn *websocket.Conn) {
	id := fmt.Sprintf("c%d", h.clientSeq.Add(1))
	c := newClient(h, conn, id)

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	logger.Log.Info("ws client connected",
		zap.String("client", id),
		zap.Int("total_clients", h.clientCount()))

	go c.writePump()
	c.readPump() // blocks until disconnect
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// unregister tears down a client's state. Called from the readPump's
// defer on disconnect.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if !h.clients[c] {
		h.mu.Unlock()
		return
	}
	delete(h.clients, c)
	for subject := range c.wantedSubjects {
		if set, ok := h.wantedBy[subject]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.wantedBy, subject)
			}
		}
	}
	h.mu.Unlock()

	c.closeSend()
	logger.Log.Info("ws client disconnected",
		zap.String("client", c.id),
		zap.Int("total_clients", h.clientCount()))
}

// dispatchCommand parses one inbound JSON frame and routes to subscribe /
// unsubscribe handlers. Unknown actions get an error reply.
func (h *Hub) dispatchCommand(c *Client, raw []byte) {
	var cmd domain.ClientAction
	if err := json.Unmarshal(raw, &cmd); err != nil {
		h.sendError(c, "invalid json: "+err.Error())
		return
	}
	switch cmd.Action {
	case "subscribe":
		h.handleSubscribe(c, cmd.Stocks, cmd.Timeframes)
	case "unsubscribe":
		h.handleUnsubscribe(c, cmd.Stocks, cmd.Timeframes)
	default:
		h.sendError(c, "unknown action: "+cmd.Action)
	}
}

func (h *Hub) handleSubscribe(c *Client, stocks, tfs []string) {
	if len(stocks) == 0 || len(tfs) == 0 {
		h.sendError(c, "subscribe requires non-empty stocks and timeframes")
		return
	}

	type pair struct{ stock, tf string }
	var added []pair

	h.mu.Lock()
	for _, s := range stocks {
		for _, tf := range tfs {
			subject := h.cfg.SubjectPrefix + "." + s + "." + tf
			if !c.wantedSubjects[subject] {
				c.wantedSubjects[subject] = true
				if _, ok := h.wantedBy[subject]; !ok {
					h.wantedBy[subject] = make(map[*Client]bool)
				}
				h.wantedBy[subject][c] = true
				added = append(added, pair{s, tf})
			}
		}
	}
	h.mu.Unlock()

	// Send Redis snapshot for each newly-wanted (stock, tf). Best-effort
	// — Redis hiccups should not bring down the connection. Run sequentially
	// to keep things deterministic; volume per connect is bounded.
	for _, p := range added {
		bar, err := h.reader.Get(context.Background(), p.stock, p.tf)
		if err != nil {
			logger.Log.Warn("snapshot read failed",
				zap.String("stock", p.stock), zap.String("tf", p.tf),
				zap.Error(err))
			continue
		}
		msg := domain.ServerMessage{
			Type:  "snapshot",
			Stock: p.stock,
			TF:    p.tf,
			Bar:   bar, // may be nil — client gets empty snapshot, that's fine
		}
		if data, err := json.Marshal(msg); err == nil {
			if !c.trySend(data) {
				logger.Log.Warn("client send buffer full on snapshot — dropping",
					zap.String("client", c.id))
				h.unregister(c)
				return
			}
		}
	}
}

func (h *Hub) handleUnsubscribe(c *Client, stocks, tfs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range stocks {
		for _, tf := range tfs {
			subject := h.cfg.SubjectPrefix + "." + s + "." + tf
			if c.wantedSubjects[subject] {
				delete(c.wantedSubjects, subject)
				if set, ok := h.wantedBy[subject]; ok {
					delete(set, c)
					if len(set) == 0 {
						delete(h.wantedBy, subject)
					}
				}
			}
		}
	}
}

// onNATS is the wildcard subscription callback. Decodes the candle payload,
// looks up clients that want this subject, and fans the JSON message out.
//
// Slow clients are unregistered on full send buffer — broadcast must not
// stall on one bad consumer.
func (h *Hub) onNATS(msg *nats.Msg) {
	subject := msg.Subject

	h.mu.RLock()
	subscribers := h.wantedBy[subject]
	if len(subscribers) == 0 {
		h.mu.RUnlock()
		return
	}
	// Snapshot the client set under the read lock so we don't mutate the
	// shared map while iterating.
	clients := make([]*Client, 0, len(subscribers))
	for c := range subscribers {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	// Re-wrap the candle payload as a ServerMessage. Decoding upfront
	// avoids paying the cost per-client.
	var bar domain.Bar
	if err := json.Unmarshal(msg.Data, &bar); err != nil {
		logger.Log.Warn("candle payload decode failed",
			zap.String("subject", subject), zap.Error(err))
		return
	}
	msgType := "update"
	if bar.Status == "closed" {
		msgType = "closed"
	}

	stock, tf := parseSubject(subject, h.cfg.SubjectPrefix)
	out, err := json.Marshal(domain.ServerMessage{
		Type:  msgType,
		Stock: stock,
		TF:    tf,
		Bar:   &bar,
	})
	if err != nil {
		return
	}

	var slowClients []*Client
	for _, c := range clients {
		if !c.trySend(out) {
			slowClients = append(slowClients, c)
		}
	}
	for _, c := range slowClients {
		logger.Log.Warn("client send buffer full — disconnecting slow consumer",
			zap.String("client", c.id))
		h.unregister(c)
	}
}

// sendError pushes a JSON error frame to one client. Best-effort.
func (h *Hub) sendError(c *Client, message string) {
	out, err := json.Marshal(domain.ServerMessage{Type: "error", Error: message})
	if err != nil {
		return
	}
	c.trySend(out)
}

// parseSubject extracts stock and tf from "<prefix>.<stock>.<tf>". Returns
// empty strings if the subject doesn't match the expected shape — caller
// passes these through to the JSON message; downstream clients can detect
// the malformed case via empty fields.
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
