// Package hub owns the per-connection lifecycle and the central fan-out
// state that maps NATS subjects to subscribed clients.
package hub

import (
	"sync"
	"time"

	"tuai/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// SendBufferSize bounds the per-client outbound queue. At 256 messages
// and ~200B JSON each, that's ~50KB stuck in memory per slow client. Once
// the buffer fills, sendNonBlocking drops the message and flags the
// client for disconnect — realtime market data is not worth backlogging.
const SendBufferSize = 256

// Connection deadlines — picked from the gorilla/websocket chat example.
const (
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = 30 * time.Second // < PongWait so server detects dead conn
	MaxMessageSize = 4 * 1024
)

// Client wraps one WebSocket connection. Lifecycle:
//
//	hub.register → readPump (reads client commands) + writePump (sends queued
//	frames)  →  hub.unregister on close
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// send is the outbound queue. Hub broadcasts and snapshot replies are
	// pushed here non-blocking. writePump drains it.
	send chan []byte

	// wantedSubjects is the set of NATS subjects this client subscribed to.
	// Mutated by the hub goroutine only.
	wantedSubjects map[string]bool

	// closed prevents double-close of the send channel; set under hub.mu.
	closed   bool
	closedMu sync.Mutex

	id string // for logging
}

// newClient is called by the hub on register.
func newClient(h *Hub, conn *websocket.Conn, id string) *Client {
	return &Client{
		hub:            h,
		conn:           conn,
		send:           make(chan []byte, SendBufferSize),
		wantedSubjects: make(map[string]bool),
		id:             id,
	}
}

// trySend attempts a non-blocking enqueue. Returns false if the buffer is
// full — caller should treat that as "client too slow, drop them".
func (c *Client) trySend(msg []byte) bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- msg:
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

// writePump moves messages from send → WS connection. Also sends ping
// frames every PingPeriod. Exits when send is closed or write fails.
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
				logger.Log.Debug("ws write error",
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

// readPump reads inbound JSON command frames and hands them to the hub.
// Also tracks pong replies for keep-alive.
//
// Exits on read error / client disconnect, then unregisters via hub.
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
				logger.Log.Debug("ws read error",
					zap.String("client", c.id), zap.Error(err))
			}
			return
		}
		c.hub.dispatchCommand(c, raw)
	}
}
