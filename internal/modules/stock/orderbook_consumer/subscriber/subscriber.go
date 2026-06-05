// Package subscriber owns the NATS JetStream connection + durable
// consumer for the order-book reconstructor.
//
// Subscribes to BOTH Type 16 (Order events) AND Type 15 (Trade match)
// from stream IDX_TICK via FilterSubjects. Type 15 is needed because
// the spec has no "filled" command in Type 16 — order removal on match
// must be derived from Trade's buyer_order_no + seller_order_no fields.
//
// Uses multi-filter (FilterSubjects []string) — single durable consumer
// receives both subject prefixes from one stream.
package subscriber

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"tuai/internal/modules/stock/iqplus_envelope"
	"tuai/pkg/logger"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Handler processes one decoded envelope.
type Handler func(ctx context.Context, env iqplus_envelope.Envelope) error

// Config contains everything needed to connect and subscribe.
type Config struct {
	URL            string
	Token          string
	ClientName     string
	Stream         string   // e.g. IDX_TICK
	DurableName    string   // e.g. orderbook-events
	FilterSubjects []string // e.g. {"idx.order.>","idx.trade.>"}
	AckWait        time.Duration
	MaxAckPending  int
	ConnectTimeout time.Duration
	ReconnectWait  time.Duration
	MaxReconnects  int
}

// DefaultConfig fills the safe operational knobs.
//
// FilterSubjects covers all three event types the engine needs:
//   - idx.order.>          Type 16 (add/cancel)
//   - idx.trade.>          Type 15 (match, drives fill)
//   - idx.status.session   Type 57 (session begin → reset + phase tag)
func DefaultConfig() Config {
	return Config{
		ClientName:     "orderbook-consumer",
		Stream:         "IDX_TICK",
		DurableName:    "orderbook-events",
		FilterSubjects: []string{"idx.order.>", "idx.trade.>", "idx.status.session"},
		AckWait:        30 * time.Second,
		MaxAckPending:  20_000, // order events bisa sangat tinggi rate
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1,
	}
}

// Stats are best-effort counters.
type Stats struct {
	Received uint64
	Acked    uint64
	Naked    uint64
	Errors   uint64
}

// Subscriber is a long-lived JetStream durable consumer.
type Subscriber struct {
	cfg     Config
	nc      *nats.Conn
	js      jetstream.JetStream
	cons    jetstream.Consumer
	consume jetstream.ConsumeContext
	handler Handler
	stats   Stats
}

func New(cfg Config, h Handler) *Subscriber {
	if h == nil {
		panic("subscriber: handler must not be nil")
	}
	if cfg.AckWait == 0 {
		cfg.AckWait = 30 * time.Second
	}
	if cfg.MaxAckPending <= 0 {
		cfg.MaxAckPending = 20_000
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
	if cfg.ClientName == "" {
		cfg.ClientName = "orderbook-consumer"
	}
	if len(cfg.FilterSubjects) == 0 {
		panic("subscriber: at least one FilterSubjects entry required")
	}
	return &Subscriber{cfg: cfg, handler: h}
}

// Start dials NATS and creates/updates the durable consumer.
func (s *Subscriber) Start(ctx context.Context) error {
	opts := []nats.Option{
		nats.Name(s.cfg.ClientName),
		nats.MaxReconnects(s.cfg.MaxReconnects),
		nats.ReconnectWait(s.cfg.ReconnectWait),
		nats.ReconnectJitter(500*time.Millisecond, 2*time.Second),
		nats.Timeout(s.cfg.ConnectTimeout),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Log.Warn("nats disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Log.Info("nats reconnected", zap.String("url", c.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Log.Info("nats connection closed")
		}),
	}
	if s.cfg.Token != "" {
		opts = append(opts, nats.Token(s.cfg.Token))
	}

	nc, err := nats.Connect(s.cfg.URL, opts...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return fmt.Errorf("jetstream new: %w", err)
	}

	cons, err := js.CreateOrUpdateConsumer(ctx, s.cfg.Stream, jetstream.ConsumerConfig{
		Durable:        s.cfg.DurableName,
		FilterSubjects: s.cfg.FilterSubjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        s.cfg.AckWait,
		MaxAckPending:  s.cfg.MaxAckPending,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
		ReplayPolicy:   jetstream.ReplayInstantPolicy,
	})
	if err != nil {
		nc.Close()
		return fmt.Errorf("create consumer %q on %q: %w",
			s.cfg.DurableName, s.cfg.Stream, err)
	}

	consume, err := cons.Consume(func(msg jetstream.Msg) {
		s.dispatch(ctx, msg)
	})
	if err != nil {
		nc.Close()
		return fmt.Errorf("start consume: %w", err)
	}

	s.nc = nc
	s.js = js
	s.cons = cons
	s.consume = consume

	logger.Log.Info("orderbook subscriber ready",
		zap.String("url", nc.ConnectedUrl()),
		zap.String("stream", s.cfg.Stream),
		zap.String("durable", s.cfg.DurableName),
		zap.Strings("filters", s.cfg.FilterSubjects))
	return nil
}

func (s *Subscriber) dispatch(ctx context.Context, msg jetstream.Msg) {
	atomic.AddUint64(&s.stats.Received, 1)

	env, err := iqplus_envelope.Unmarshal(msg.Data())
	if err != nil {
		atomic.AddUint64(&s.stats.Errors, 1)
		logger.Log.Warn("envelope decode error",
			zap.String("subject", msg.Subject()), zap.Error(err))
		_ = msg.Ack()
		return
	}

	if err := s.handler(ctx, env); err != nil {
		atomic.AddUint64(&s.stats.Naked, 1)
		if nakErr := msg.Nak(); nakErr != nil && !errors.Is(nakErr, nats.ErrConnectionClosed) {
			logger.Log.Debug("nak error", zap.Error(nakErr))
		}
		return
	}

	atomic.AddUint64(&s.stats.Acked, 1)
	if ackErr := msg.Ack(); ackErr != nil && !errors.Is(ackErr, nats.ErrConnectionClosed) {
		logger.Log.Debug("ack error", zap.Error(ackErr))
	}
}

// Snapshot returns a copy of current counters.
func (s *Subscriber) Snapshot() Stats {
	return Stats{
		Received: atomic.LoadUint64(&s.stats.Received),
		Acked:    atomic.LoadUint64(&s.stats.Acked),
		Naked:    atomic.LoadUint64(&s.stats.Naked),
		Errors:   atomic.LoadUint64(&s.stats.Errors),
	}
}

// Close stops consuming and drains the NATS connection.
func (s *Subscriber) Close() {
	if s.consume != nil {
		s.consume.Stop()
	}
	if s.nc != nil {
		if err := s.nc.Drain(); err != nil {
			logger.Log.Warn("nats drain error", zap.Error(err))
		}
	}
}
