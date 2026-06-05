// Command ws-gateway exposes WebSocket endpoints that fan realtime
// trading data from NATS out to browser-side clients.
//
// Routes:
//
//	/healthz       liveness probe (also reports v1 channel registry)
//	/ws/candles    LEGACY candle-only endpoint — backward compat for old FE
//	/ws/v1         UNIFIED channel-multiplexed endpoint (orderbook + candle
//	               + future channels), see docs/orderbook/protocol.md §3
//
// Deploy (production):
//
//	./deployments/kubernetes/ws-gateway/deploy.sh <tag>
//
//	Manifest:    deployments/kubernetes/ws-gateway/deployment.yaml
//	Dockerfile:  deployments/docker/ws-gateway.Dockerfile (if present;
//	             otherwise built via the generic consumer.Dockerfile path)
//	Env secret:  tuai-be-ws-gateway-env (namespace tuai)
//
// Local dev:
//
//	go run ./cmd/ws-gateway
//
// Required environment variables:
//
//	NATS_URL                   e.g. nats://10.10.8.2:4222
//	REDIS_ADDR                 host:port (shared by candle + orderbook
//	                           readers, different DBs per channel)
//
// Optional / has defaults:
//
//	NATS_TOKEN                 bearer token for NATS
//	NATS_CLIENT_NAME           default ws-gateway-<hostname>
//
//	REDIS_PASSWORD             shared
//	CANDLE_REDIS_DB            default 11 (matches running-trade-consumer)
//	CANDLE_REDIS_KEY_PREFIX    default "candle"
//	CANDLE_SUBJECT_PREFIX      default "idx.candle"
//	ORDERBOOK_REDIS_DB         default 9 (matches orderbook-consumer)
//	ORDERBOOK_REDIS_KEY_PREFIX default "orderbook"
//	ORDERBOOK_SUBJECT_PREFIX   default "idx.orderbook"
//
//	HTTP_PORT                  default 8081
//	WS_ALLOWED_ORIGINS         csv; empty = allow all (dev only)
//	WS_HANDSHAKE_TIMEOUT       default 10s
//
//	ENV                        development|production
//
// AUTH NOTE: this binary has NO authentication on either WS endpoint.
// Production must front it with a reverse proxy that validates session
// before passing through. The proxy may inject X-User-ID for analytics.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	wsgw "tuai/internal/modules/stock/ws_gateway"
	"tuai/internal/modules/stock/ws_gateway/handler"
	"tuai/internal/modules/stock/ws_gateway/hub"
	"tuai/internal/modules/stock/ws_gateway/snapshot"
	v1 "tuai/internal/modules/stock/ws_gateway/v1"
	candlev1 "tuai/internal/modules/stock/ws_gateway/v1/channels/candle"
	orderbookv1 "tuai/internal/modules/stock/ws_gateway/v1/channels/orderbook"
	"tuai/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func init() {
	time.Local = time.UTC
}

func main() {
	loadEnvFile()

	env := envOr("ENV", "production")
	if err := logger.Initialize(env); err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := loadConfig()
	if err != nil {
		logger.Log.Fatal("config error", zap.Error(err))
	}

	logger.Log.Info("ws-gateway starting",
		zap.String("nats_url", cfg.NATS.URL),
		zap.String("redis_addr", cfg.V1CandleRedis.Addr),
		zap.Int("candle_redis_db", cfg.V1CandleRedis.DB),
		zap.Int("orderbook_redis_db", cfg.V1OrderbookRedis.DB),
		zap.String("candle_subject_prefix", cfg.V1Candle.SubjectPrefix),
		zap.String("orderbook_subject_prefix", cfg.V1Orderbook.SubjectPrefix))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := wsgw.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init", zap.Error(err))
	}
	defer mod.Shutdown()

	if err := mod.Start(ctx); err != nil {
		logger.Log.Fatal("hub start", zap.Error(err))
	}
	logger.Log.Info("ws-gateway hubs ready",
		zap.Strings("v1_channels", mod.V1Hub.Channels()))

	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	mod.LegacyHandler.Register(router.Group("/"))

	port := envOr("HTTP_PORT", "8081")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Log.Info("http server listening", zap.String("port", port))
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("http server error", zap.Error(err))
			stop()
		}
	}()

	<-ctx.Done()
	logger.Log.Info("ws-gateway shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Warn("http server shutdown error", zap.Error(err))
	}
	logger.Log.Info("ws-gateway stopped cleanly")
}

func loadConfig() (wsgw.Config, error) {
	// --- NATS (shared) ---
	natsCfg := wsgw.DefaultNATSConfig()
	natsCfg.URL = mustEnv("NATS_URL")
	natsCfg.Token = os.Getenv("NATS_TOKEN")
	natsCfg.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())

	// --- Redis (shared addr + password, separate DB per channel) ---
	redisAddr := mustEnv("REDIS_ADDR")
	redisPass := os.Getenv("REDIS_PASSWORD")

	// --- Legacy candle path (REDIS_DB / REDIS_KEY_PREFIX kept for backward
	// compat with old env files; falls through to CANDLE_REDIS_* if set). ---
	legacyCandleRedis := snapshot.DefaultConfig()
	legacyCandleRedis.Addr = redisAddr
	legacyCandleRedis.Password = redisPass
	legacyCandleRedis.DB = envInt("REDIS_DB", envInt("CANDLE_REDIS_DB", 11))
	legacyCandleRedis.KeyPrefix = envOr("REDIS_KEY_PREFIX",
		envOr("CANDLE_REDIS_KEY_PREFIX", legacyCandleRedis.KeyPrefix))
	legacyCandleRedis.ClientName = natsCfg.ClientName

	legacyHubCfg := hub.Config{
		SubjectPrefix: envOr("CANDLE_SUBJECT_PREFIX", "idx.candle"),
	}
	legacyHandlerCfg := handler.Config{
		AllowedOrigins:   parseList(os.Getenv("WS_ALLOWED_ORIGINS")),
		HandshakeTimeout: envDuration("WS_HANDSHAKE_TIMEOUT", 10*time.Second),
	}

	// --- v1 candle channel (same Redis host, default db 11) ---
	v1CandleRedis := candlev1.DefaultRedisConfig()
	v1CandleRedis.Addr = redisAddr
	v1CandleRedis.Password = redisPass
	v1CandleRedis.DB = envInt("CANDLE_REDIS_DB", 11)
	v1CandleRedis.KeyPrefix = envOr("CANDLE_REDIS_KEY_PREFIX", v1CandleRedis.KeyPrefix)
	v1CandleRedis.ClientName = natsCfg.ClientName + "-v1-candle"
	v1CandleCfg := candlev1.DefaultConfig()
	v1CandleCfg.SubjectPrefix = envOr("CANDLE_SUBJECT_PREFIX", v1CandleCfg.SubjectPrefix)

	// --- v1 orderbook channel (same Redis host, default db 9) ---
	v1OrderbookRedis := orderbookv1.DefaultRedisConfig()
	v1OrderbookRedis.Addr = redisAddr
	v1OrderbookRedis.Password = redisPass
	v1OrderbookRedis.DB = envInt("ORDERBOOK_REDIS_DB", 9)
	v1OrderbookRedis.KeyPrefix = envOr("ORDERBOOK_REDIS_KEY_PREFIX", v1OrderbookRedis.KeyPrefix)
	v1OrderbookRedis.ClientName = natsCfg.ClientName + "-v1-orderbook"
	v1OrderbookCfg := orderbookv1.DefaultConfig()
	v1OrderbookCfg.SubjectPrefix = envOr("ORDERBOOK_SUBJECT_PREFIX", v1OrderbookCfg.SubjectPrefix)

	v1HubCfg := v1.DefaultHubConfig()
	v1HubCfg.AllowedOrigins = parseList(os.Getenv("WS_ALLOWED_ORIGINS"))
	v1HubCfg.HandshakeTimeout = envDuration("WS_HANDSHAKE_TIMEOUT", v1HubCfg.HandshakeTimeout)

	return wsgw.Config{
		NATS:             natsCfg,
		LegacyCandle:     legacyCandleRedis,
		LegacyHub:        legacyHubCfg,
		LegacyHandler:    legacyHandlerCfg,
		V1Hub:            v1HubCfg,
		V1Candle:         v1CandleCfg,
		V1CandleRedis:    v1CandleRedis,
		V1Orderbook:      v1OrderbookCfg,
		V1OrderbookRedis: v1OrderbookRedis,
	}, nil
}

// loadEnvFile mirrors the per-service convention used by the other consumers.
//
// Lookup order:
//  1. $WS_GATEWAY_ENV_FILE          (explicit)
//  2. ./cmd/ws-gateway/.env         (repo root via `go run`)
//  3. <exe-dir>/.env
//  4. <exe-dir>/ws-gateway.env
func loadEnvFile() {
	if explicit := os.Getenv("WS_GATEWAY_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load WS_GATEWAY_ENV_FILE=%s: %v\n", explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}
	candidates := []string{"cmd/ws-gateway/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "ws-gateway.env"),
		)
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			fmt.Fprintf(os.Stderr,
				"warning: %s exists but failed to load: %v\n", path, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", path)
		return
	}
}

func defaultClientName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return "ws-gateway-" + host
}

func parseList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Log.Fatal("missing required env var", zap.String("key", key))
	}
	return v
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		logger.Log.Warn("invalid int env, using default",
			zap.String("key", key), zap.String("value", v), zap.Int("default", def))
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		logger.Log.Warn("invalid duration env, using default",
			zap.String("key", key), zap.String("value", v),
			zap.Duration("default", def))
		return def
	}
	return d
}
