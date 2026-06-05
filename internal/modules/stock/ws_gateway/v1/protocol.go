// Package v1 implements the unified WebSocket protocol for the trading
// frontend: a single endpoint /ws/v1 that multiplexes many realtime
// channels (orderbook, candle, trades, quote, ...) over one connection.
//
// Channel-registry pattern: each realtime feed is a Channel module that
// implements the Channel interface — it owns its NATS subscription, its
// per-client subscriber index, and its own command argument shape. The
// Hub is a thin router that dispatches inbound commands by channel name
// and manages connection lifecycle.
//
// Why this shape (vs separate /ws/<channel> routes):
//   - FE keeps ONE WebSocket per browser tab (lower fd/RAM cost server-side,
//     simpler reconnect logic client-side, easier per-user rate limiting).
//   - Adding a new channel = drop a new package under channels/ and register
//     it in main — no new HTTP route, no new ingress entry, no new auth
//     wiring.
//   - Matches the industry-standard shape used by Binance, Coinbase, Bybit.
//
// Wire protocol — see also docs/orderbook/protocol.md §3 for the orderbook
// channel's specifics. Common envelope:
//
//	# Client → Server
//	{ "op":"subscribe",  "channel":"<name>", <channel-specific args>... }
//	{ "op":"unsubscribe","channel":"<name>", <channel-specific args>... }
//	{ "op":"resync",     "channel":"<name>", <channel-specific args>... }
//	{ "op":"ping" }                                  # no channel
//
//	# Server → Client (channel-tagged so FE can route to the right consumer)
//	{ "type":"hello",    "server":"<id>",   "ts":<ms> }
//	{ "type":"snapshot", "channel":"<n>",   ... }
//	{ "type":"update",   "channel":"<n>",   ... }   # for candle bars
//	{ "type":"delta",    "channel":"<n>",   ... }   # for orderbook diffs
//	{ "type":"reset",    "channel":"<n>",   ... }
//	{ "type":"error",    "code":"<C>",      "msg":"..." }
//	{ "type":"pong",     "ts":<ms> }
package v1

import (
	"context"
	"encoding/json"
)

// Op constants enumerate inbound command verbs.
const (
	OpSubscribe   = "subscribe"
	OpUnsubscribe = "unsubscribe"
	OpResync      = "resync"
	OpPing        = "ping"
)

// MsgType constants enumerate outbound message types.
const (
	TypeHello    = "hello"
	TypeSnapshot = "snapshot"
	TypeUpdate   = "update"
	TypeDelta    = "delta"
	TypeReset    = "reset"
	TypeError    = "error"
	TypePong     = "pong"
)

// CommandEnvelope is the minimum decode of an inbound frame — used by the
// Hub to route to the right channel before that channel parses its own
// args from the raw JSON.
type CommandEnvelope struct {
	Op      string `json:"op"`
	Channel string `json:"channel,omitempty"`
}

// HelloMessage is sent once when the WS upgrade completes. ServerID helps
// support tickets correlate which pod a user landed on (load-balanced
// across multiple ws-gateway replicas).
type HelloMessage struct {
	Type     string `json:"type"`
	ServerID string `json:"server"`
	TS       int64  `json:"ts"`
}

// PongMessage answers an application-level {"op":"ping"}.
type PongMessage struct {
	Type string `json:"type"`
	TS   int64  `json:"ts"`
}

// ErrorMessage carries non-fatal protocol errors (bad JSON, unknown
// channel, validation failure). The connection stays open — clients can
// resubscribe correctly.
type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	Channel string `json:"channel,omitempty"` // optional context
}

// Channel is the contract every realtime feed implements. The Hub never
// knows what a channel does internally — it only routes commands and
// triggers lifecycle hooks.
//
// Concurrency: methods may be called from many goroutines (commands from
// per-client read pumps, NATS callbacks, disconnect cleanup). The channel
// is responsible for its own synchronization.
type Channel interface {
	// Name returns the wire-protocol identifier — must match what clients
	// send in `{"channel":"<name>"}`. Stable, lowercase, single token.
	Name() string

	// Start initializes external subscriptions (NATS, etc). Called once
	// after the hub is wired and before the HTTP server accepts traffic.
	Start(ctx context.Context) error

	// Stop tears down external subscriptions and releases resources.
	// Called once on shutdown; idempotent.
	Stop()

	// HandleSubscribe parses channel-specific args from `raw` and registers
	// the client for live updates. May also push initial snapshot(s) to
	// the client via c.Send().
	HandleSubscribe(c *Client, raw json.RawMessage) error

	// HandleUnsubscribe removes the client's registration.
	HandleUnsubscribe(c *Client, raw json.RawMessage) error

	// HandleResync re-sends a fresh snapshot for the client's current
	// subscription set without changing it. Used by FE on sequence gap
	// detection.
	HandleResync(c *Client, raw json.RawMessage) error

	// OnDisconnect cleans up any per-client state when the connection
	// closes. Called once per disconnect after the client has stopped
	// reading; safe to mutate channel-internal subscriber maps.
	OnDisconnect(c *Client)
}

// Error codes — kept short, screaming-snake, so clients can branch on them
// without parsing free-text Msg.
const (
	ErrBadJSON         = "BAD_JSON"
	ErrUnknownChannel  = "UNKNOWN_CHANNEL"
	ErrUnknownOp       = "UNKNOWN_OP"
	ErrBadArgs         = "BAD_ARGS"
	ErrTooManySubs     = "TOO_MANY_SUBSCRIPTIONS"
	ErrSnapshotFail    = "SNAPSHOT_FAIL"
	ErrInternal        = "INTERNAL"
)
