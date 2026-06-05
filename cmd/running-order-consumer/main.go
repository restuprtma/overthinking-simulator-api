// Command running-order-consumer subscribes to idx.order.> (Type 16,
// live) on JetStream and writes every order event to the QuestDB
// `running_orders` table.
//
// Mirrors the running_trades half of running-trade-consumer for orders. Live
// broker codes are masked as "--" — the broker-real archive lives in
// `orders` (cmd/resend-order-consumer).
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/running-order-consumer-freebsd-amd64 ./cmd/running-order-consumer
//
// Required environment variables:
//
//	NATS_URL                e.g. nats://10.10.8.2:4222
//	QUESTDB_ADDRESS         host:port (HTTP ILP, e.g. 10.10.8.51:9000)
//	QUESTDB_TABLE           destination table (e.g. running_orders)
//
// Optional / has defaults:
//
//	NATS_TOKEN              bearer token
//	NATS_CLIENT_NAME        default running-order-consumer-<hostname>
//	NATS_STREAM             default IDX_TICK
//	NATS_DURABLE            default running-order-consumer
//	NATS_FILTER_SUBJECT     default idx.order.>
//	NATS_ACK_WAIT           default 30s
//	NATS_MAX_ACK_PENDING    default 5000
//
//	QUESTDB_TOKEN           bearer token
//	QUESTDB_AUTH_USER       basic auth user
//	QUESTDB_AUTH_PASSWORD
//	QUESTDB_AUTO_FLUSH_ROWS default 1000
//	QUESTDB_AUTO_FLUSH_INT  default 500ms
//
//	STATS_INTERVAL          default 30s
//	FLUSH_ON_TICK           default true
//	FLUSH_TIMEOUT           default 10s
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
	"syscall"
	"time"

	orderSink "tuai/internal/modules/stock/resend_order_consumer/sink"
	running_order_consumer "tuai/internal/modules/stock/running_order_consumer"
	"tuai/internal/modules/stock/running_order_consumer/service"
	"tuai/internal/modules/stock/running_order_consumer/subscriber"
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

	logger.Log.Info("running-order-consumer starting",
		zap.String("nats_url", cfg.Subscriber.URL),
		zap.String("nats_stream", cfg.Subscriber.Stream),
		zap.String("nats_durable", cfg.Subscriber.DurableName),
		zap.String("nats_filter", cfg.Subscriber.FilterSubject),
		zap.String("questdb_addr", cfg.QuestDB.Address),
		zap.String("questdb_table", cfg.QuestDB.Table))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := running_order_consumer.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init failed", zap.Error(err))
	}

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("consumer exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("running-order-consumer stopped cleanly")
}

func loadConfig() (running_order_consumer.Config, error) {
	sub := subscriber.DefaultConfig()
	sub.URL = mustEnv("NATS_URL")
	sub.Token = os.Getenv("NATS_TOKEN")
	sub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	sub.Stream = envOr("NATS_STREAM", sub.Stream)
	sub.DurableName = envOr("NATS_DURABLE", sub.DurableName)
	sub.FilterSubject = envOr("NATS_FILTER_SUBJECT", sub.FilterSubject)
	sub.AckWait = envDuration("NATS_ACK_WAIT", sub.AckWait)
	sub.MaxAckPending = envInt("NATS_MAX_ACK_PENDING", sub.MaxAckPending)

	qdb := orderSink.DefaultQuestDBConfig()
	qdb.Address = mustEnv("QUESTDB_ADDRESS")
	qdb.Table = mustEnv("QUESTDB_TABLE")
	qdb.Token = os.Getenv("QUESTDB_TOKEN")
	qdb.BasicAuthUser = os.Getenv("QUESTDB_AUTH_USER")
	qdb.BasicAuthPassword = os.Getenv("QUESTDB_AUTH_PASSWORD")
	qdb.AutoFlushRows = envInt("QUESTDB_AUTO_FLUSH_ROWS", qdb.AutoFlushRows)
	qdb.AutoFlushInterval = envDuration("QUESTDB_AUTO_FLUSH_INT", qdb.AutoFlushInterval)

	svcCfg := service.Config{
		StatsTick:    envDuration("STATS_INTERVAL", 30*time.Second),
		FlushOnTick:  envBool("FLUSH_ON_TICK", true),
		FlushTimeout: envDuration("FLUSH_TIMEOUT", 10*time.Second),
	}

	return running_order_consumer.Config{
		Subscriber: sub,
		QuestDB:    qdb,
		Service:    svcCfg,
	}, nil
}

// loadEnvFile loads this service's .env file. Per-service isolation.
//
// Lookup order:
//  1. $RUNNING_ORDER_CONSUMER_ENV_FILE
//  2. ./cmd/running-order-consumer/.env
//  3. <exe-dir>/.env
//  4. <exe-dir>/running-order-consumer.env
func loadEnvFile() {
	if explicit := os.Getenv("RUNNING_ORDER_CONSUMER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load RUNNING_ORDER_CONSUMER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{"cmd/running-order-consumer/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "running-order-consumer.env"),
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
	return "running-order-consumer-" + host
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

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		logger.Log.Warn("invalid bool env, using default",
			zap.String("key", key), zap.String("value", v), zap.Bool("default", def))
		return def
	}
	return b
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
