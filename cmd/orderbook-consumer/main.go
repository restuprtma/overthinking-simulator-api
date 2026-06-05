// Command orderbook-consumer reconstructs the IDX order book in real time
// from per-event sources (Type 16 Order + Type 15 Trade) and persists
// per-stock state to Redis on a sub-second cadence + publishes deltas to
// NATS for ws-gateway fanout.
//
// Deploy (production):
//
//	# Build + push image via Docker Hub Autobuild, apply k8s manifest:
//	./deployments/kubernetes/orderbook-consumer/deploy.sh <tag>
//
//	Manifest:    deployments/kubernetes/orderbook-consumer/deployment.yaml
//	Dockerfile:  deployments/docker/orderbook-consumer.Dockerfile
//	Env secret:  tuai-be-orderbook-consumer-env (namespace tuai)
//	Replicas:    1 (stateful — see deployment.yaml comments)
//
// Local dev:
//
//	go run ./cmd/orderbook-consumer
//
// Required environment variables:
//
//	NATS_URL                e.g. nats://10.10.8.2:4222
//	REDIS_ADDR              host:port
//
// Optional / has defaults:
//
//	NATS_TOKEN              bearer token
//	NATS_CLIENT_NAME        default orderbook-consumer-<hostname>
//	NATS_STREAM             default IDX_TICK
//	NATS_DURABLE            default orderbook-events
//	NATS_FILTER_SUBJECTS    csv override; default
//	                        "idx.order.>,idx.trade.>,idx.status.session"
//	NATS_ACK_WAIT           default 30s
//	NATS_MAX_ACK_PENDING    default 20000
//
//	NATS_PUBLISH_URL        publisher conn for orderbook delta + reset;
//	                        defaults to NATS_URL when unset (same broker)
//	NATS_PUBLISH_TOKEN      defaults to NATS_TOKEN
//	NATS_PUBLISH_PREFIX     default idx.orderbook (subject prefix)
//
//	REDIS_PASSWORD          default ""
//	REDIS_DB                default 9
//	REDIS_KEY_PREFIX        default orderbook
//	REDIS_STATE_TTL         default 25h
//
//	SNAPSHOT_INTERVAL       default 100ms (sub-second redis flush)
//	SNAPSHOT_FLUSH_TIMEOUT  default 5s
//
//	STATS_INTERVAL          default 30s
//	ENV                     development|production
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	orderbook_consumer "tuai/internal/modules/stock/orderbook_consumer"
	"tuai/internal/modules/stock/orderbook_consumer/service"
	"tuai/internal/modules/stock/orderbook_consumer/sink"
	"tuai/internal/modules/stock/orderbook_consumer/snapshotter"
	"tuai/internal/modules/stock/orderbook_consumer/subscriber"
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

	logger.Log.Info("orderbook-consumer starting (event-sourced)",
		zap.String("nats_url", cfg.Subscriber.URL),
		zap.String("nats_stream", cfg.Subscriber.Stream),
		zap.String("nats_durable", cfg.Subscriber.DurableName),
		zap.Strings("nats_filters", cfg.Subscriber.FilterSubjects),
		zap.String("nats_publish_url", cfg.NATS.URL),
		zap.String("nats_publish_prefix", cfg.NATS.SubjectPrefix),
		zap.String("redis_addr", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
		zap.String("redis_key_prefix", cfg.Redis.KeyPrefix),
		zap.Duration("snapshot_interval", cfg.Snapshotter.Interval))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := orderbook_consumer.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init failed", zap.Error(err))
	}

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("consumer exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("orderbook-consumer stopped cleanly")
}

func loadConfig() (orderbook_consumer.Config, error) {
	sub := subscriber.DefaultConfig()
	sub.URL = mustEnv("NATS_URL")
	sub.Token = os.Getenv("NATS_TOKEN")
	sub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	sub.Stream = envOr("NATS_STREAM", sub.Stream)
	sub.DurableName = envOr("NATS_DURABLE", sub.DurableName)
	if csv := os.Getenv("NATS_FILTER_SUBJECTS"); csv != "" {
		sub.FilterSubjects = parseCSV(csv)
	}
	sub.AckWait = envDuration("NATS_ACK_WAIT", sub.AckWait)
	sub.MaxAckPending = envInt("NATS_MAX_ACK_PENDING", sub.MaxAckPending)

	redisCfg := sink.DefaultConfig()
	redisCfg.Addr = mustEnv("REDIS_ADDR")
	redisCfg.Password = os.Getenv("REDIS_PASSWORD")
	redisCfg.DB = envInt("REDIS_DB", 9)
	redisCfg.KeyPrefix = envOr("REDIS_KEY_PREFIX", redisCfg.KeyPrefix)
	redisCfg.StateTTL = envDuration("REDIS_STATE_TTL", redisCfg.StateTTL)
	redisCfg.ClientName = sub.ClientName

	natsCfg := sink.DefaultNATSConfig()
	// Publisher conn defaults to the same broker as the subscriber. Allow
	// override in case the two are separated (e.g. publisher on edge VM).
	natsCfg.URL = envOr("NATS_PUBLISH_URL", sub.URL)
	natsCfg.Token = envOr("NATS_PUBLISH_TOKEN", sub.Token)
	natsCfg.SubjectPrefix = envOr("NATS_PUBLISH_PREFIX", natsCfg.SubjectPrefix)
	natsCfg.ClientName = sub.ClientName + "-publisher"

	snapCfg := snapshotter.DefaultConfig()
	snapCfg.Interval = envDuration("SNAPSHOT_INTERVAL", snapCfg.Interval)
	snapCfg.FlushTimeout = envDuration("SNAPSHOT_FLUSH_TIMEOUT", snapCfg.FlushTimeout)

	svcCfg := service.Config{
		StatsTick: envDuration("STATS_INTERVAL", 30*time.Second),
	}

	return orderbook_consumer.Config{
		Subscriber:  sub,
		Redis:       redisCfg,
		NATS:        natsCfg,
		Snapshotter: snapCfg,
		Service:     svcCfg,
	}, nil
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// loadEnvFile loads this service's .env file. Per-service isolation.
//
// Lookup order:
//  1. $ORDERBOOK_CONSUMER_ENV_FILE
//  2. ./cmd/orderbook-consumer/.env
//  3. <exe-dir>/.env
//  4. <exe-dir>/orderbook-consumer.env
func loadEnvFile() {
	if explicit := os.Getenv("ORDERBOOK_CONSUMER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load ORDERBOOK_CONSUMER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{"cmd/orderbook-consumer/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "orderbook-consumer.env"),
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
	return "orderbook-consumer-" + host
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
