// Package handler exposes the Gin endpoints for ws-gateway.
//
// Routes:
//
//	GET /healthz                       — liveness probe (also reports v1
//	                                      channel registry)
//	GET /ws/candles                    — LEGACY candle-only WebSocket;
//	                                      backward compat
//	GET /ws/v1                         — UNIFIED channel-multiplexed
//	                                      WebSocket
//	GET /api/v1/orderbook/:code        — REST snapshot fallback for
//	                                      SSR / cold-start / debug; reads
//	                                      the same Redis db 9 as the
//	                                      orderbook WS channel
//
// AUTH: Phase 1 has NO authentication on any endpoint. Production
// deployment MUST front this service with a reverse proxy that validates
// session/JWT before forwarding.
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tuai/internal/modules/stock/ws_gateway/hub"
	v1 "tuai/internal/modules/stock/ws_gateway/v1"
	orderbookv1 "tuai/internal/modules/stock/ws_gateway/v1/channels/orderbook"
	"tuai/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Config holds tunables for the upgrader + the handler.
type Config struct {
	// CheckOrigin: comma-separated list of allowed Origin headers. Empty
	// means "allow all" — for dev only. Reverse proxy in production
	// should be the actual origin gate.
	AllowedOrigins   []string
	HandshakeTimeout time.Duration
}

// Handler bundles BOTH the legacy candle hub and the v1 unified hub,
// plus a read-only orderbook snapshot reader for the REST fallback.
// New deployments wire all three; routing is split by URL path.
type Handler struct {
	legacy          *hub.Hub
	v1Hub           *v1.Hub
	orderbookReader *orderbookv1.Reader
	upgrader        websocket.Upgrader
}

// New builds a Handler for the legacy candle path only — kept for binary
// compatibility with the old wire path. Production callers should prefer
// NewFull to also serve /ws/v1 and the REST snapshot endpoint.
func New(h *hub.Hub, cfg Config) *Handler {
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = 10 * time.Second
	}
	return &Handler{
		legacy: h,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: cfg.HandshakeTimeout,
			ReadBufferSize:   1024,
			WriteBufferSize:  4096,
			CheckOrigin:      makeOriginCheck(cfg.AllowedOrigins),
		},
	}
}

// NewWithV1 builds a Handler that serves /ws/candles (legacy) and
// /ws/v1 (unified). REST snapshot endpoint is NOT registered — call
// NewFull instead if you also want it.
func NewWithV1(legacy *hub.Hub, v1Hub *v1.Hub, cfg Config) *Handler {
	h := New(legacy, cfg)
	h.v1Hub = v1Hub
	return h
}

// NewFull builds a Handler that serves all available routes:
// /ws/candles, /ws/v1, and the REST snapshot endpoint backed by the
// orderbook Redis reader.
func NewFull(legacy *hub.Hub, v1Hub *v1.Hub, orderbookReader *orderbookv1.Reader, cfg Config) *Handler {
	h := NewWithV1(legacy, v1Hub, cfg)
	h.orderbookReader = orderbookReader
	return h
}

// Register attaches all available routes to a Gin router group.
//
// Optional routes (only registered when their dep is wired):
//   - /ws/v1                      when v1Hub != nil
//   - /api/v1/orderbook/:code     when orderbookReader != nil
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/healthz", h.healthz)
	r.GET("/ws/candles", h.candles)
	if h.v1Hub != nil {
		r.GET("/ws/v1", h.v1)
	}
	if h.orderbookReader != nil {
		r.GET("/api/v1/orderbook/:code", h.orderbookSnapshot)
	}
}

// orderbookSnapshot returns a one-shot snapshot of the depth book for
// one stock. Same data shape the /ws/v1 channel sends on subscribe —
// suitable for SSR (Next.js getServerSideProps) or debug.
//
// Query params:
//   - depth (optional, default 10, server-capped at 30)
//
// Response: 200 + Snapshot JSON, or 404 if the stock has no state in
// Redis (engine never saw it in this run / TTL expired).
func (h *Handler) orderbookSnapshot(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "stock code required"})
		return
	}
	depth := 10
	if raw := c.Query("depth"); raw != "" {
		if d, err := strconv.Atoi(raw); err == nil && d > 0 {
			depth = d
		}
	}
	if depth > 30 {
		depth = 30
	}

	snap, err := h.orderbookReader.Get(c.Request.Context(), code, depth)
	if err != nil {
		logger.Log.Warn("orderbook snapshot REST read failed",
			zap.String("stock", code), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "snapshot read failed"})
		return
	}
	if snap == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "no orderbook state for " + code,
		})
		return
	}
	// Short cache — orderbook is hot data, but a 1s edge cache absorbs
	// repeated REST polls cheaply.
	c.Header("Cache-Control", "public, max-age=1")
	c.JSON(http.StatusOK, snap)
}

func (h *Handler) healthz(c *gin.Context) {
	status := gin.H{"status": "ok"}
	if h.v1Hub != nil {
		s := h.v1Hub.Stats()
		status["v1"] = gin.H{
			"channels":    s.Channels,
			"clients":     s.Clients,
			"connections": s.Connections,
			"commands":    s.Commands,
		}
	}
	c.JSON(http.StatusOK, status)
}

// candles upgrades the connection and hands it to the legacy candle hub.
func (h *Handler) candles(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Log.Warn("ws upgrade failed (candles)",
			zap.String("remote", c.ClientIP()), zap.Error(err))
		return
	}
	// Register blocks until the connection closes — keeps the handler
	// goroutine alive so Gin doesn't recycle the response writer.
	h.legacy.Register(conn)
}

// v1 upgrades the connection via the v1 Hub's own upgrader (which has
// its own AllowedOrigins config). The v1 Hub.ServeHTTP handles upgrade
// + connection registration + blocking until disconnect.
func (h *Handler) v1(c *gin.Context) {
	h.v1Hub.ServeHTTP(c.Writer, c.Request)
}

// makeOriginCheck returns a CheckOrigin func. Empty allow list permits
// any origin (dev mode); explicit list does exact match.
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
