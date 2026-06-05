// Package ws_gateway wires the NATS subscriber, Redis snapshot readers,
// and Gin HTTP server that fans realtime trading data out to WebSocket
// clients.
//
// Two protocols served side-by-side:
//
//	/ws/candles  — legacy channel-specific endpoint; FE candle clients
//	               keep working unchanged during the v1 migration window.
//
//	/ws/v1       — unified channel-multiplexed endpoint. New channels
//	               (orderbook, candle, trades, quote, …) live as plugins
//	               under internal/.../ws_gateway/v1/channels and register
//	               themselves with the v1 Hub at startup.
//
// Architecture notes (see also docs/orderbook/protocol.md):
//   - One NATS connection shared by all channels (legacy + v1).
//   - One Redis client per logical data source (candle in db 11, orderbook
//     in db 9). go-redis clients are bound to a single DB, so we don't
//     share.
//   - The Hub itself never reads NATS or Redis; channel plugins own their
//     subscriptions and snapshot readers.
//
// Auth: Phase 1 ships WITH NO authentication on either endpoint.
// Production must front this service with a reverse proxy that gates the
// upgrade and (optionally) injects an X-User-ID header for analytics.
package ws_gateway

import (
	"context"
	"fmt"
	"time"

	"tuai/internal/modules/stock/ws_gateway/handler"
	"tuai/internal/modules/stock/ws_gateway/hub"
	"tuai/internal/modules/stock/ws_gateway/snapshot"
	v1 "tuai/internal/modules/stock/ws_gateway/v1"
	candlev1 "tuai/internal/modules/stock/ws_gateway/v1/channels/candle"
	orderbookv1 "tuai/internal/modules/stock/ws_gateway/v1/channels/orderbook"
	"tuai/pkg/logger"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Config is the union of all sub-configs. NATS connection is shared
// across legacy + v1 paths.
type Config struct {
	NATS NATSConfig

	// LegacyCandle is the existing /ws/candles endpoint config (Redis +
	// subject prefix). Kept for backward compat.
	LegacyCandle  snapshot.Config
	LegacyHub     hub.Config
	LegacyHandler handler.Config

	// V1Hub is the unified /ws/v1 hub config (origins, handshake timeout).
	V1Hub v1.HubConfig

	// V1Candle wires the candle channel plugin under v1.
	V1Candle      candlev1.Config
	V1CandleRedis candlev1.RedisConfig

	// V1Orderbook wires the orderbook channel plugin under v1.
	V1Orderbook      orderbookv1.Config
	V1OrderbookRedis orderbookv1.RedisConfig
}

// NATSConfig holds connection params for the realtime bus.
type NATSConfig struct {
	URL            string
	Token          string
	ClientName     string
	ConnectTimeout time.Duration
	ReconnectWait  time.Duration
	MaxReconnects  int
}

// DefaultNATSConfig returns operational defaults.
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{
		ClientName:     "ws-gateway",
		ConnectTimeout: 5 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1,
	}
}

// Module bundles the long-lived components.
type Module struct {
	NATSConn *nats.Conn

	// Legacy candle endpoint.
	LegacyReader  *snapshot.Reader
	LegacyHub     *hub.Hub
	LegacyHandler *handler.Handler

	// v1 unified endpoint.
	V1Hub             *v1.Hub
	V1CandleReader    *candlev1.Reader
	V1OrderbookReader *orderbookv1.Reader
}

// Initialize connects to NATS + Redis (one client per DB) up-front and
// wires the channels into the v1 Hub. Fail-fast on bad config.
func Initialize(ctx context.Context, cfg Config) (*Module, error) {
	if cfg.NATS.URL == "" {
		return nil, fmt.Errorf("nats url required")
	}
	if cfg.LegacyCandle.Addr == "" {
		return nil, fmt.Errorf("legacy candle redis addr required")
	}
	if cfg.V1CandleRedis.Addr == "" {
		return nil, fmt.Errorf("v1 candle redis addr required")
	}
	if cfg.V1OrderbookRedis.Addr == "" {
		return nil, fmt.Errorf("v1 orderbook redis addr required")
	}

	nc, err := dialNATS(cfg.NATS)
	if err != nil {
		return nil, fmt.Errorf("nats: %w", err)
	}

	// Legacy candle wiring (unchanged behaviour).
	legacyReader, err := snapshot.New(ctx, cfg.LegacyCandle)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("legacy candle redis: %w", err)
	}
	legacyHub := hub.New(cfg.LegacyHub, nc, legacyReader)

	// v1 readers — separate clients because go-redis is per-DB.
	v1CandleReader, err := candlev1.NewReader(ctx, cfg.V1CandleRedis)
	if err != nil {
		nc.Close()
		_ = legacyReader.Close()
		return nil, fmt.Errorf("v1 candle redis: %w", err)
	}
	v1OrderbookReader, err := orderbookv1.NewReader(ctx, cfg.V1OrderbookRedis)
	if err != nil {
		nc.Close()
		_ = legacyReader.Close()
		_ = v1CandleReader.Close()
		return nil, fmt.Errorf("v1 orderbook redis: %w", err)
	}

	// v1 hub + channel plugins.
	v1Hub := v1.NewHub(cfg.V1Hub)
	v1Hub.Register(candlev1.New(nc, v1CandleReader, cfg.V1Candle))
	v1Hub.Register(orderbookv1.New(nc, v1OrderbookReader, cfg.V1Orderbook))

	// HTTP handler serves legacy /ws/candles, unified /ws/v1, and the
	// REST snapshot /api/v1/orderbook/:code (powered by the same Redis
	// reader as the orderbook WS channel).
	legacyHandler := handler.NewFull(legacyHub, v1Hub, v1OrderbookReader, cfg.LegacyHandler)

	return &Module{
		NATSConn:          nc,
		LegacyReader:      legacyReader,
		LegacyHub:         legacyHub,
		LegacyHandler:     legacyHandler,
		V1Hub:             v1Hub,
		V1CandleReader:    v1CandleReader,
		V1OrderbookReader: v1OrderbookReader,
	}, nil
}

// Start begins NATS subscriptions for legacy + v1. After this returns
// the gateway is ready to accept WebSocket clients.
func (m *Module) Start(ctx context.Context) error {
	if err := m.LegacyHub.Start(ctx); err != nil {
		return fmt.Errorf("legacy hub: %w", err)
	}
	if err := m.V1Hub.Start(ctx); err != nil {
		return fmt.Errorf("v1 hub: %w", err)
	}
	return nil
}

// Shutdown stops both hubs and drains the NATS connection. Safe to call
// once.
func (m *Module) Shutdown() {
	if m.V1Hub != nil {
		m.V1Hub.Stop()
	}
	if m.LegacyHub != nil {
		m.LegacyHub.Stop()
	}
	if m.NATSConn != nil {
		if err := m.NATSConn.Drain(); err != nil {
			logger.Log.Warn("ws-gateway nats drain", zap.Error(err))
		}
	}
	if m.LegacyReader != nil {
		_ = m.LegacyReader.Close()
	}
	if m.V1CandleReader != nil {
		_ = m.V1CandleReader.Close()
	}
	if m.V1OrderbookReader != nil {
		_ = m.V1OrderbookReader.Close()
	}
}

func dialNATS(cfg NATSConfig) (*nats.Conn, error) {
	if cfg.ClientName == "" {
		cfg.ClientName = "ws-gateway"
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
			logger.Log.Warn("ws-gateway nats disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Log.Info("ws-gateway nats reconnected",
				zap.String("url", c.ConnectedUrl()))
		}),
	}
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}
	return nats.Connect(cfg.URL, opts...)
}
