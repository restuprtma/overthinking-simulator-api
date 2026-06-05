// Command quote-consumer subscribes to idx.quote.> on JetStream and
// maintains the latest known state per stock as a Redis hash
// `quote:<stock>` with named FID aliases (prev_close, last_price, ...).
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/quote-consumer-freebsd-amd64 ./cmd/quote-consumer
//
// Required environment variables:
//
//	NATS_URL                e.g. nats://10.10.8.2:4222
//	REDIS_ADDR              host:port (e.g. localhost:6379)
//
// Optional / has defaults:
//
//	NATS_TOKEN              bearer token (blank = no auth)
//	NATS_CLIENT_NAME        default quote-consumer-<hostname>
//	NATS_STREAM             default IDX_QUOTE
//	NATS_DURABLE            default quote-state-cache
//	NATS_FILTER_SUBJECT     default idx.quote.>
//	NATS_ACK_WAIT           default 30s
//	NATS_MAX_ACK_PENDING    default 10000
//
//	REDIS_PASSWORD          default ""
//	REDIS_DB                default 12 (different from running-trade consumer's 11)
//	REDIS_KEY_PREFIX        default quote
//	REDIS_STATE_TTL         default 25h
//
//	STATS_INTERVAL          default 30s
//	ENV                     development|production (logger format)
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	quote_consumer "tuai/internal/modules/stock/quote_consumer"
	"tuai/internal/modules/stock/quote_consumer/service"
	"tuai/internal/modules/stock/quote_consumer/sink"
	"tuai/internal/modules/stock/quote_consumer/subscriber"
	"tuai/pkg/logger"

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

	logger.Log.Info("quote-consumer starting",
		zap.String("nats_url", cfg.Subscriber.URL),
		zap.String("nats_stream", cfg.Subscriber.Stream),
		zap.String("nats_durable", cfg.Subscriber.DurableName),
		zap.String("nats_filter", cfg.Subscriber.FilterSubject),
		zap.String("redis_addr", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
		zap.String("redis_key_prefix", cfg.Redis.KeyPrefix))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := quote_consumer.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init failed", zap.Error(err))
	}

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("consumer exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("quote-consumer stopped cleanly")
}

func loadConfig() (quote_consumer.Config, error) {
	sub := subscriber.DefaultConfig()
	sub.URL = mustEnv("NATS_URL")
	sub.Token = os.Getenv("NATS_TOKEN")
	sub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	sub.Stream = envOr("NATS_STREAM", sub.Stream)
	sub.DurableName = envOr("NATS_DURABLE", sub.DurableName)
	sub.FilterSubject = envOr("NATS_FILTER_SUBJECT", sub.FilterSubject)
	sub.AckWait = envDuration("NATS_ACK_WAIT", sub.AckWait)
	sub.MaxAckPending = envInt("NATS_MAX_ACK_PENDING", sub.MaxAckPending)

	redisCfg := sink.DefaultConfig()
	redisCfg.Addr = mustEnv("REDIS_ADDR")
	redisCfg.Password = os.Getenv("REDIS_PASSWORD")
	redisCfg.DB = envInt("REDIS_DB", 12)
	redisCfg.KeyPrefix = envOr("REDIS_KEY_PREFIX", redisCfg.KeyPrefix)
	redisCfg.StateTTL = envDuration("REDIS_STATE_TTL", redisCfg.StateTTL)
	redisCfg.ClientName = sub.ClientName

	svcCfg := service.Config{
		StatsTick: envDuration("STATS_INTERVAL", 30*time.Second),
	}

	return quote_consumer.Config{
		Subscriber: sub,
		Redis:      redisCfg,
		Service:    svcCfg,
	}, nil
}

// loadEnvFile loads this service's .env file. Mirrors the pattern in
// cmd/iqplus-publisher and cmd/running-trade-consumer — per-service isolation.
//
// Lookup order:
//  1. $QUOTE_CONSUMER_ENV_FILE         (explicit path)
//  2. ./cmd/quote-consumer/.env        (repo root via `go run`)
//  3. <exe-dir>/.env                   (FreeBSD deploy)
//  4. <exe-dir>/quote-consumer.env     (FreeBSD deploy, sibling style)
func loadEnvFile() {
	if explicit := os.Getenv("QUOTE_CONSUMER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load QUOTE_CONSUMER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{"cmd/quote-consumer/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "quote-consumer.env"),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s exists but failed to load: %v\n", path, err)
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
	return "quote-consumer-" + host
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
			zap.String("key", key), zap.String("value", v), zap.Duration("default", def))
		return def
	}
	return d
}
