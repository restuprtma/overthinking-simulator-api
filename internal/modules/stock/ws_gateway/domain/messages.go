// Package domain holds the wire format for the ws-gateway protocol — both
// inbound (client → server) subscribe/unsubscribe commands and outbound
// (server → client) snapshot/update/error messages.
//
// Protocol summary:
//
//	# Client → Server
//	{"action":"subscribe",   "stocks":["BBCA","BMRI"], "timeframes":["1m"]}
//	{"action":"unsubscribe", "stocks":["BBCA"],        "timeframes":["1m"]}
//
//	# Server → Client
//	{"type":"snapshot", "stock":"BBCA", "tf":"1m", "bar": {...}}    -- on subscribe (from Redis)
//	{"type":"update",   "stock":"BBCA", "tf":"1m", "bar": {...}}    -- live (from NATS)
//	{"type":"closed",   "stock":"BBCA", "tf":"1m", "bar": {...}}    -- bucket finalised
//	{"type":"error",    "error":"..."}
//
// Bar field names match the running_trade_consumer/sink/nats_candle.go payload
// shape so the gateway can pass NATS bytes through after a single decode.
package domain

// ClientAction is one inbound message.
type ClientAction struct {
	Action     string   `json:"action"`     // "subscribe" | "unsubscribe"
	Stocks     []string `json:"stocks"`
	Timeframes []string `json:"timeframes"`
}

// ServerMessage is one outbound message.
type ServerMessage struct {
	Type  string `json:"type"`            // "snapshot" | "update" | "closed" | "error"
	Stock string `json:"stock,omitempty"`
	TF    string `json:"tf,omitempty"`
	Bar   *Bar   `json:"bar,omitempty"`
	Error string `json:"error,omitempty"`
}

// Bar matches the candle publisher's JSON shape exactly.
type Bar struct {
	Stock   string  `json:"stock"`
	TF      string  `json:"tf"`
	OpenTS  int64   `json:"open_ts"`
	CloseTS int64   `json:"close_ts"`
	Open    float64 `json:"o"`
	High    float64 `json:"h"`
	Low     float64 `json:"l"`
	Close   float64 `json:"c"`
	Volume  int64   `json:"v"`
	Trades  int64   `json:"trades"`
	Status  string  `json:"status"` // "live" | "closed"
}
