package v1

import (
	"sync"
	"time"

	"tuai/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// SendBufferSize bounds the per-client outbound queue. With the unified
// protocol a single client subscribes to N channels × M stocks, so the
// buffer must absorb burst-period messages from all of them. 512 covers
// ~5s of backlog at 100 msg/s without dropping; past that the client is
// considered too slow and dropped (preserves freshness over delivery).
const SendBufferSize = 512

// WS-level deadlines. PingPeriod < PongWait so the server detects dead
// connections before the client read pump times out reading.
const (
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = 30 * time.Second
	MaxMessageSize = 8 * 1024 // larger than the channel-specific hubs because args may be richer in v1
)

// Client wraps one WebSocket connection. Created by the Hub on upgrade;
// torn down by the readPump's defer on disconnect.
//
// Channels see Client only as a sink — they call Send() to push frames
// and read ID() / userID for logging. Per-channel subscriber state lives
// inside each channel, not here.
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// send is the outbound queue. Send() pushes non-blocking; writePump
	// drains. Buffered to avoid head-of-line blocking between channels.
	send chan []byte

	id string

	// userID is the authenticated user this connection belongs to. Empty
	// for unauthenticated connections (Phase 1 — gateway has no auth yet).
	// Channels can use it for permission checks or analytics tags.
	userID string

	closed   bool
	closedMu sync.Mutex
}

// newClient builds a Client. Called only by the Hub on upgrade.
func newClient(h *Hub, conn *websocket.Conn, id, userID string) *Client {
	return &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, SendBufferSize),
		id:     id,
		userID: userID,
	}
}

// ID returns the connection's short identifier used in logs.
func (c *Client) ID() string { return c.id }

// UserID returns the authenticated user (empty if unauthenticated).
func (c *Client) UserID() string { return c.userID }

// Send pushes a frame onto the outbound queue. Non-blocking; returns
// false if the buffer is full (caller should treat that as "drop this
// slow client"). Safe to call from any goroutine.
func (c *Client) Send(payload []byte) bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// closeSend marks the client closed and closes the send channel so the
// writePump exits. Idempotent.
func (c *Client) closeSend() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
}

// writePump drains send → WS connection. Also emits periodic WS ping
// control frames for keepalive. Exits when send closes or write fails.
func (c *Client) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				logger.Log.Debug("ws/v1 write error",
					zap.String("client", c.id), zap.Error(err))
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump parses inbound JSON frames and routes them via the hub. Exits
// on read error / disconnect, then triggers unregister.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseAbnormalClosure) {
				logger.Log.Debug("ws/v1 read error",
					zap.String("client", c.id), zap.Error(err))
			}
			return
		}
		c.hub.dispatchCommand(c, raw)
	}
}
