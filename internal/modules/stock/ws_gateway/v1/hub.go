package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"tuai/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// HubConfig controls the v1 Hub.
type HubConfig struct {
	// AllowedOrigins is the WS CheckOrigin allow-list (empty = allow all,
	// suitable for dev only — production should front behind an ingress
	// that gates origin headers).
	AllowedOrigins []string
	// HandshakeTimeout caps the WS upgrade negotiation.
	HandshakeTimeout time.Duration
}

// DefaultHubConfig returns operational defaults.
func DefaultHubConfig() HubConfig {
	return HubConfig{
		HandshakeTimeout: 10 * time.Second,
	}
}

// Hub is the central registry — channels register themselves, clients
// connect, inbound commands route by channel name. The Hub itself does
// not subscribe to NATS or read Redis; channels do.
type Hub struct {
	cfg HubConfig

	mu       sync.RWMutex
	channels map[string]Channel
	clients  map[*Client]bool

	upgrader websocket.Upgrader

	clientSeq atomic.Uint64
	serverID  string

	started  atomic.Bool
	statConn uint64
	statCmd  uint64
}

// NewHub constructs a Hub. Channels are registered via Register() before
// Start() so the hub knows the full surface at startup.
func NewHub(cfg HubConfig) *Hub {
	host, _ := os.Hostname()
	if host == "" {
		host = "ws-gateway"
	}
	return &Hub{
		cfg:      cfg,
		channels: make(map[string]Channel),
		clients:  make(map[*Client]bool),
		serverID: host,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: cfg.HandshakeTimeout,
			ReadBufferSize:   1024,
			WriteBufferSize:  4096,
			CheckOrigin:      makeOriginCheck(cfg.AllowedOrigins),
		},
	}
}

// Register adds a channel to the hub. Must be called before Start().
// Panics on duplicate name — that would be a wiring bug worth surfacing
// immediately.
func (h *Hub) Register(ch Channel) {
	if h.started.Load() {
		panic("v1 hub: Register called after Start")
	}
	name := ch.Name()
	if name == "" {
		panic("v1 hub: channel returned empty Name()")
	}
	if _, exists := h.channels[name]; exists {
		panic("v1 hub: duplicate channel " + name)
	}
	h.channels[name] = ch
}

// Channels returns a snapshot of registered channel names — useful for
// /healthz output and tests.
func (h *Hub) Channels() []string {
	out := make([]string, 0, len(h.channels))
	for name := range h.channels {
		out = append(out, name)
	}
	return out
}

// Start fires the lifecycle hooks on every registered channel. If any
// channel fails to start, already-started channels are stopped and the
// error is returned.
func (h *Hub) Start(ctx context.Context) error {
	if !h.started.CompareAndSwap(false, true) {
		return fmt.Errorf("v1 hub: already started")
	}
	started := make([]Channel, 0, len(h.channels))
	for name, ch := range h.channels {
		if err := ch.Start(ctx); err != nil {
			for _, sc := range started {
				sc.Stop()
			}
			return fmt.Errorf("start channel %q: %w", name, err)
		}
		started = append(started, ch)
		logger.Log.Info("ws/v1 channel started", zap.String("channel", name))
	}
	return nil
}

// Stop tears down every channel and disconnects all clients.
func (h *Hub) Stop() {
	for name, ch := range h.channels {
		ch.Stop()
		logger.Log.Info("ws/v1 channel stopped", zap.String("channel", name))
	}
	h.mu.Lock()
	for c := range h.clients {
		c.closeSend()
	}
	h.clients = map[*Client]bool{}
	h.mu.Unlock()
}

// ServeHTTP is the HTTP handler that upgrades to WebSocket. Wire under
// /ws/v1 — see handler.go for the Gin-side registration.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Warn("ws/v1 upgrade failed",
			zap.String("remote", r.RemoteAddr), zap.Error(err))
		return
	}
	// Phase 1: no auth. Production must front with reverse proxy that
	// pre-authenticates and optionally injects a header — read userID
	// here when that arrives.
	userID := r.Header.Get("X-User-ID") // empty if no proxy
	h.register(conn, userID)
}

// register wires a fresh connection into the hub, sends the hello frame,
// and runs the read+write pumps. Blocks until the read pump returns
// (client disconnect).
func (h *Hub) register(conn *websocket.Conn, userID string) {
	atomic.AddUint64(&h.statConn, 1)
	id := fmt.Sprintf("c%d", h.clientSeq.Add(1))
	c := newClient(h, conn, id, userID)

	h.mu.Lock()
	h.clients[c] = true
	count := len(h.clients)
	h.mu.Unlock()

	logger.Log.Info("ws/v1 client connected",
		zap.String("client", id),
		zap.String("user", userID),
		zap.Int("total", count))

	// Send hello so FE knows the upgrade succeeded and which pod it landed
	// on (helps support correlate logs to user reports).
	hello, _ := json.Marshal(HelloMessage{
		Type:     TypeHello,
		ServerID: h.serverID,
		TS:       time.Now().UnixMilli(),
	})
	c.Send(hello)

	go c.writePump()
	c.readPump() // blocks until disconnect
}

// unregister tears down per-client state across all channels then closes
// the send channel so writePump exits.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if !h.clients[c] {
		h.mu.Unlock()
		return
	}
	delete(h.clients, c)
	count := len(h.clients)
	h.mu.Unlock()

	// Notify every channel so they can drop this client from their
	// subscriber maps. Channels must be tolerant of being called even if
	// the client never subscribed to them (no-op in that case).
	for _, ch := range h.channels {
		ch.OnDisconnect(c)
	}

	c.closeSend()
	logger.Log.Info("ws/v1 client disconnected",
		zap.String("client", c.id),
		zap.Int("total", count))
}

// dispatchCommand parses one inbound frame envelope and routes to the
// named channel. Channel-specific arg parsing is delegated to the
// channel's Handle* methods (they re-parse `raw` into their own shape).
func (h *Hub) dispatchCommand(c *Client, raw []byte) {
	atomic.AddUint64(&h.statCmd, 1)

	var env CommandEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		h.sendError(c, ErrBadJSON, "", "invalid JSON: "+err.Error())
		return
	}

	// `ping` is the only op that doesn't require a channel.
	if env.Op == OpPing {
		pong, _ := json.Marshal(PongMessage{Type: TypePong, TS: time.Now().UnixMilli()})
		c.Send(pong)
		return
	}

	if env.Channel == "" {
		h.sendError(c, ErrBadArgs, "", "missing `channel` field")
		return
	}

	ch, ok := h.channels[env.Channel]
	if !ok {
		h.sendError(c, ErrUnknownChannel, env.Channel,
			fmt.Sprintf("no such channel %q", env.Channel))
		return
	}

	var err error
	switch env.Op {
	case OpSubscribe:
		err = ch.HandleSubscribe(c, raw)
	case OpUnsubscribe:
		err = ch.HandleUnsubscribe(c, raw)
	case OpResync:
		err = ch.HandleResync(c, raw)
	default:
		h.sendError(c, ErrUnknownOp, env.Channel,
			fmt.Sprintf("unknown op %q", env.Op))
		return
	}
	if err != nil {
		h.sendError(c, ErrBadArgs, env.Channel, err.Error())
	}
}

// sendError pushes a typed error frame to one client. Best-effort.
func (h *Hub) sendError(c *Client, code, channel, msg string) {
	body, err := json.Marshal(ErrorMessage{
		Type: TypeError, Code: code, Msg: msg, Channel: channel,
	})
	if err != nil {
		return
	}
	c.Send(body)
}

// Stats returns operational counters for /healthz or metrics export.
type Stats struct {
	Clients     int
	Connections uint64 // total ever
	Commands    uint64
	Channels    []string
}

// Stats returns counters.
func (h *Hub) Stats() Stats {
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()
	return Stats{
		Clients:     count,
		Connections: atomic.LoadUint64(&h.statConn),
		Commands:    atomic.LoadUint64(&h.statCmd),
		Channels:    h.Channels(),
	}
}

// makeOriginCheck returns the CheckOrigin func used by the WS upgrader.
// Empty allow list permits any origin (dev mode); explicit list does
// case-insensitive exact match.
func makeOriginCheck(allowed []string) func(*http.Request) bool {
	if len(allowed) == 0 {
		return func(_ *http.Request) bool { return true }
	}
	allowSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowSet[o] = true
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return allowSet[origin]
	}
}
