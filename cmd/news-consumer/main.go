// Command news-consumer subscribes to idx.news.> on JetStream,
// reassembles multi-packet news frames, and inserts the complete
// articles into MongoDB.
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/news-consumer-freebsd-amd64 ./cmd/news-consumer
//
// Required environment variables:
//
//	NATS_URL        e.g. nats://10.10.8.2:4222
//	MONGO_URI       e.g. mongodb://user:pass@host:27017/?authSource=admin
//
// Optional / has defaults:
//
//	NATS_TOKEN              bearer token (blank = no auth)
//	NATS_CLIENT_NAME        default news-consumer-<hostname>
//	NATS_STREAM             default IDX_NEWS
//	NATS_DURABLE            default news-indexer
//	NATS_FILTER_SUBJECT     default idx.news.>
//	NATS_ACK_WAIT           default 60s
//	NATS_MAX_ACK_PENDING    default 500
//
//	MONGO_DATABASE          default tuai
//	MONGO_COLLECTION        default news
//	MONGO_CONNECT_TIMEOUT   default 5s
//	MONGO_WRITE_TIMEOUT     default 5s
//
//	NEWS_BUFFER_STALE_AFTER default 10m
//	NEWS_BUFFER_SWEEP       default 1m
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

	news_consumer "tuai/internal/modules/stock/news_consumer"
	"tuai/internal/modules/stock/news_consumer/assembler"
	"tuai/internal/modules/stock/news_consumer/service"
	"tuai/internal/modules/stock/news_consumer/sink"
	"tuai/internal/modules/stock/news_consumer/subscriber"
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

	logger.Log.Info("news-consumer starting",
		zap.String("nats_url", cfg.Subscriber.URL),
		zap.String("nats_stream", cfg.Subscriber.Stream),
		zap.String("nats_durable", cfg.Subscriber.DurableName),
		zap.String("nats_filter", cfg.Subscriber.FilterSubject),
		zap.String("mongo_db", cfg.Mongo.Database),
		zap.String("mongo_collection", cfg.Mongo.Collection))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := news_consumer.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init failed", zap.Error(err))
	}

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("consumer exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("news-consumer stopped cleanly")
}

func loadConfig() (news_consumer.Config, error) {
	sub := subscriber.DefaultConfig()
	sub.URL = mustEnv("NATS_URL")
	sub.Token = os.Getenv("NATS_TOKEN")
	sub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	sub.Stream = envOr("NATS_STREAM", sub.Stream)
	sub.DurableName = envOr("NATS_DURABLE", sub.DurableName)
	sub.FilterSubject = envOr("NATS_FILTER_SUBJECT", sub.FilterSubject)
	sub.AckWait = envDuration("NATS_ACK_WAIT", sub.AckWait)
	sub.MaxAckPending = envInt("NATS_MAX_ACK_PENDING", sub.MaxAckPending)

	mongoCfg := sink.DefaultConfig()
	mongoCfg.URI = mustEnv("MONGO_URI")
	mongoCfg.Database = envOr("MONGO_DATABASE", mongoCfg.Database)
	mongoCfg.Collection = envOr("MONGO_COLLECTION", mongoCfg.Collection)
	mongoCfg.ConnectTimeout = envDuration("MONGO_CONNECT_TIMEOUT", mongoCfg.ConnectTimeout)
	mongoCfg.WriteTimeout = envDuration("MONGO_WRITE_TIMEOUT", mongoCfg.WriteTimeout)

	asmCfg := assembler.DefaultConfig()
	asmCfg.StaleAfter = envDuration("NEWS_BUFFER_STALE_AFTER", asmCfg.StaleAfter)
	asmCfg.SweepInterval = envDuration("NEWS_BUFFER_SWEEP", asmCfg.SweepInterval)

	svcCfg := service.Config{
		StatsTick: envDuration("STATS_INTERVAL", 30*time.Second),
	}

	return news_consumer.Config{
		Subscriber: sub,
		Mongo:      mongoCfg,
		Assembler:  asmCfg,
		Service:    svcCfg,
	}, nil
}

// loadEnvFile loads this service's .env file. Mirrors the pattern in the
// other consumers — per-service isolation.
//
// Lookup order:
//  1. $NEWS_CONSUMER_ENV_FILE
//  2. ./cmd/news-consumer/.env
//  3. <exe-dir>/.env
//  4. <exe-dir>/news-consumer.env
func loadEnvFile() {
	if explicit := os.Getenv("NEWS_CONSUMER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load NEWS_CONSUMER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{"cmd/news-consumer/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "news-consumer.env"),
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
	return "news-consumer-" + host
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
