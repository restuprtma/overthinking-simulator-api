package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tuai/internal/modules/stock/running_trade_consumer/aggregator"
	"tuai/pkg/logger"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NATSCandleConfig holds the broadcast publisher params. Uses NATS core
// (not JetStream) — candle updates are realtime fan-out, no durability or
// replay needed. WS clients that miss messages re-fetch from Redis on
// reconnect.
//
// SubjectPrefix is joined with "." + stock + "." + timeframe. Default
// "idx.candle". Final subjects look like "idx.candle.BBCA.1m".
type NATSCandleConfig struct {
	URL            string
	Token          string
	ClientName     string
	SubjectPrefix  string        // default "idx.candle"
	ConnectTimeout time.Duration // default 5s
	ReconnectWait  time.Duration // default 2s
	MaxReconnects  int           // default -1 (infinite)
}

// DefaultNATSCandleConfig returns operational defaults.
func DefaultNATSCandleConfig() NATSCandleConfig {
	return NATSCandleConfig{
		ClientName:     "running-trade-candle-publisher",
		SubjectPrefix:  "idx.candle",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1,
	}
}

// NATSCandleSink publishes one JSON message per Update / Close to NATS core
// subject `<prefix>.<stock>.<tf>`.
//
// Failure semantics: this sink is best-effort. Publish errors are LOGGED
// but not returned, so the aggregator's main path is never blocked by a
// broadcast hiccup. Storage (QuestDB, Redis) already succeeded; broadcast
// is informational and WS clients can recover via Redis snapshot on
// reconnect.
type NATSCandleSink struct {
	cfg NATSCandleConfig
	nc  *nats.Conn
}

// NewNATSCandle dials NATS and verifies the connection. Caller is expected
// to call Shutdown() during graceful shutdown.
func NewNATSCandle(_ context.Context, cfg NATSCandleConfig) (*NATSCandleSink, error) {
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "idx.candle"
	}
	if cfg.ClientName == "" {
		cfg.ClientName = "running-trade-candle-publisher"
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
		nats.ReconnectJitter(500*time.Millisecond, 2*time.Second),
		nats.Timeout(cfg.ConnectTimeout),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Log.Warn("candle-publisher nats disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Log.Info("candle-publisher nats reconnected",
				zap.String("url", c.ConnectedUrl()))
		}),
	}
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect (candle publisher): %w", err)
	}
	return &NATSCandleSink{cfg: cfg, nc: nc}, nil
}

// candlePayload is what gets JSON-serialised onto the wire. Field names
// are deliberately short (`o/h/l/c/v`) — these go to browser WS clients
// at high frequency, every byte counts at the fan-out layer.
type candlePayload struct {
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

func (s *NATSCandleSink) toPayload(b aggregator.Bar, status string) candlePayload {
	return candlePayload{
		Stock:   b.Stock,
		TF:      b.Timeframe,
		OpenTS:  b.OpenTime.Unix(),
		CloseTS: b.CloseTime.Unix(),
		Open:    b.Open,
		High:    b.High,
		Low:     b.Low,
		Close:   b.Close,
		Volume:  b.Volume,
		Trades:  b.Trades,
		Status:  status,
	}
}

// Update fires on every tick. Best-effort publish — errors logged, not
// returned, to keep the aggregator path moving.
func (s *NATSCandleSink) Update(_ context.Context, b aggregator.Bar) error {
	s.publish(b, "live")
	return nil
}

// Close fires when a bucket finalises. Same best-effort semantics.
func (s *NATSCandleSink) Close(_ context.Context, b aggregator.Bar) error {
	s.publish(b, "closed")
	return nil
}

func (s *NATSCandleSink) publish(b aggregator.Bar, status string) {
	subject := s.cfg.SubjectPrefix + "." + b.Stock + "." + b.Timeframe
	payload, err := json.Marshal(s.toPayload(b, status))
	if err != nil {
		// json.Marshal of a fixed struct shouldn't realistically fail —
		// log loudly if it does.
		logger.Log.Error("candle payload marshal failed",
			zap.String("subject", subject), zap.Error(err))
		return
	}
	if err := s.nc.Publish(subject, payload); err != nil {
		logger.Log.Warn("candle publish failed",
			zap.String("subject", subject), zap.Error(err))
	}
}

// Shutdown drains the NATS connection. Safe to call once.
func (s *NATSCandleSink) Shutdown() {
	if s.nc != nil {
		if err := s.nc.Drain(); err != nil {
			logger.Log.Warn("candle-publisher drain error", zap.Error(err))
		}
	}
}
