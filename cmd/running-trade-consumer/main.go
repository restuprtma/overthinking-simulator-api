// Command running-trade-consumer subscribes to idx.trade.> on JetStream, writes
// raw ticks into QuestDB, and maintains live 1-minute bars in Redis.
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/running-trade-consumer-freebsd-amd64 ./cmd/running-trade-consumer
//
// Required environment variables:
//
//	NATS_URL                e.g. nats://10.10.8.2:4222
//	REDIS_ADDR              host:port (e.g. localhost:6379)
//	QUESTDB_ADDRESS         host:port (HTTP ILP, e.g. localhost:9000)
//
// Optional / has defaults:
//
//	NATS_TOKEN              bearer token (blank = no auth)
//	NATS_CLIENT_NAME        default running-trade-consumer-<hostname>
//	NATS_STREAM             default IDX_TICK
//	NATS_DURABLE            default running-trade-consumer
//	NATS_FILTER_SUBJECT     default idx.trade.>
//	NATS_ACK_WAIT           default 30s
//	NATS_MAX_ACK_PENDING    default 5000
//
//	REDIS_PASSWORD          default ""
//	REDIS_DB                default 11
//	REDIS_KEY_PREFIX        default candle
//	REDIS_BAR_TTL           default 25h
//
//	QUESTDB_TABLE           default running_trades
//	QUESTDB_TOKEN           bearer token
//	QUESTDB_AUTH_USER       basic auth user (alternative to token)
//	QUESTDB_AUTH_PASSWORD
//	QUESTDB_AUTO_FLUSH_ROWS default 1000
//	QUESTDB_AUTO_FLUSH_INT  default 500ms
//
//	OHLCV_TIMEFRAMES        default 1m   (comma-separated Go durations,
//	                        e.g. "1m,5m,15m,1h,4h" — aggregator emits one
//	                        bar update per tick per timeframe)
//	OHLCV_TIMEFRAME         legacy singular form; if set and OHLCV_TIMEFRAMES
//	                        is empty, treated as a single-tf shortcut
//	STATS_INTERVAL          default 30s
//
//	# Candle broadcast (for ws-gateway). Disabled by default.
//	ENABLE_CANDLE_PUBLISHER default false
//	CANDLE_NATS_URL         (defaults to NATS_URL when EnableNATSCand=true)
//	CANDLE_NATS_TOKEN       (defaults to NATS_TOKEN)
//	CANDLE_SUBJECT_PREFIX   default idx.candle  (final subject = <prefix>.<stock>.<tf>)
//
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
	"strings"
	"syscall"
	"time"

	running_trade_consumer "tuai/internal/modules/stock/running_trade_consumer"
	"tuai/internal/modules/stock/running_trade_consumer/aggregator"
	"tuai/internal/modules/stock/running_trade_consumer/service"
	"tuai/internal/modules/stock/running_trade_consumer/sink"
	"tuai/internal/modules/stock/running_trade_consumer/subscriber"
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

	logger.Log.Info("running-trade-consumer starting",
		zap.String("nats_url", cfg.Subscriber.URL),
		zap.String("nats_stream", cfg.Subscriber.Stream),
		zap.String("nats_durable", cfg.Subscriber.DurableName),
		zap.String("nats_filter", cfg.Subscriber.FilterSubject),
		zap.String("redis_addr", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
		zap.String("questdb_addr", cfg.QuestDB.Address),
		zap.String("questdb_table", cfg.QuestDB.Table),
		zap.Strings("timeframes", timeframeLabels(cfg.Aggregator.Timeframes)))

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mod, err := running_trade_consumer.Initialize(ctx, cfg)
	if err != nil {
		logger.Log.Fatal("module init failed", zap.Error(err))
	}

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("consumer exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("running-trade-consumer stopped cleanly")
}

func loadConfig() (running_trade_consumer.Config, error) {
	sub := subscriber.DefaultConfig()
	sub.URL = mustEnv("NATS_URL")
	sub.Token = os.Getenv("NATS_TOKEN")
	sub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	sub.Stream = envOr("NATS_STREAM", sub.Stream)
	sub.DurableName = envOr("NATS_DURABLE", sub.DurableName)
	sub.FilterSubject = envOr("NATS_FILTER_SUBJECT", sub.FilterSubject)
	sub.AckWait = envDuration("NATS_ACK_WAIT", sub.AckWait)
	sub.MaxAckPending = envInt("NATS_MAX_ACK_PENDING", sub.MaxAckPending)

	redisCfg := sink.DefaultRedisConfig()
	redisCfg.Addr = mustEnv("REDIS_ADDR")
	redisCfg.Password = os.Getenv("REDIS_PASSWORD")
	redisCfg.DB = envInt("REDIS_DB", 11)
	redisCfg.KeyPrefix = envOr("REDIS_KEY_PREFIX", redisCfg.KeyPrefix)
	redisCfg.BarTTL = envDuration("REDIS_BAR_TTL", redisCfg.BarTTL)
	redisCfg.ClientName = sub.ClientName

	qdbCfg := sink.DefaultQuestDBConfig()
	qdbCfg.Address = mustEnv("QUESTDB_ADDRESS")
	qdbCfg.Table = envOr("QUESTDB_TABLE", qdbCfg.Table)
	qdbCfg.Token = os.Getenv("QUESTDB_TOKEN")
	qdbCfg.BasicAuthUser = os.Getenv("QUESTDB_AUTH_USER")
	qdbCfg.BasicAuthPassword = os.Getenv("QUESTDB_AUTH_PASSWORD")
	qdbCfg.AutoFlushRows = envInt("QUESTDB_AUTO_FLUSH_ROWS", qdbCfg.AutoFlushRows)
	qdbCfg.AutoFlushInterval = envDuration("QUESTDB_AUTO_FLUSH_INT", qdbCfg.AutoFlushInterval)

	tfList := os.Getenv("OHLCV_TIMEFRAMES")
	if tfList == "" {
		// Backward-compat: honour the singular env when set, default 1m.
		if singular := os.Getenv("OHLCV_TIMEFRAME"); singular != "" {
			tfList = singular
		} else {
			tfList = "1m"
		}
	}
	tfs, err := parseTimeframes(tfList)
	if err != nil {
		return running_trade_consumer.Config{}, fmt.Errorf("OHLCV_TIMEFRAMES %q: %w", tfList, err)
	}
	aggCfg := aggregator.Config{Timeframes: tfs}

	svcCfg := service.Config{
		StatsTick: envDuration("STATS_INTERVAL", 30*time.Second),
	}

	candleCfg := sink.DefaultNATSCandleConfig()
	enableCandle := envBool("ENABLE_CANDLE_PUBLISHER", false)
	if enableCandle {
		candleCfg.URL = envOr("CANDLE_NATS_URL", sub.URL)
		candleCfg.Token = envOr("CANDLE_NATS_TOKEN", sub.Token)
		candleCfg.ClientName = envOr("CANDLE_NATS_CLIENT_NAME", "candle-pub-"+defaultClientName())
		candleCfg.SubjectPrefix = envOr("CANDLE_SUBJECT_PREFIX", candleCfg.SubjectPrefix)
	}

	return running_trade_consumer.Config{
		Subscriber:     sub,
		QuestDB:        qdbCfg,
		Redis:          redisCfg,
		NATSCandle:     candleCfg,
		Aggregator:     aggCfg,
		Service:        svcCfg,
		EnableNATSCand: enableCandle,
	}, nil
}

// loadEnvFile loads this service's .env file. Mirrors the pattern in
// cmd/iqplus-publisher — per-service isolation, no fallback to root .env.
//
// Lookup order:
//  1. $RUNNING_TRADE_CONSUMER_ENV_FILE        (explicit)
//  2. ./cmd/running-trade-consumer/.env       (repo root via `go run`)
//  3. <exe-dir>/.env
//  4. <exe-dir>/running-trade-consumer.env
func loadEnvFile() {
	if explicit := os.Getenv("RUNNING_TRADE_CONSUMER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load RUNNING_TRADE_CONSUMER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{"cmd/running-trade-consumer/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "running-trade-consumer.env"),
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

// parseTimeframes accepts a comma-separated list of Go durations
// (e.g. "1m,5m,15m,1h,4h") and returns aggregator Timeframes with their
// canonical short labels filled in.
func parseTimeframes(s string) ([]aggregator.Timeframe, error) {
	parts := strings.Split(s, ",")
	out := make([]aggregator.Timeframe, 0, len(parts))
	seen := make(map[time.Duration]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		d, err := time.ParseDuration(p)
		if err != nil {
			return nil, fmt.Errorf("invalid timeframe %q: %w", p, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("timeframe %q must be > 0", p)
		}
		if seen[d] {
			continue // dedup
		}
		seen[d] = true
		out = append(out, aggregator.Timeframe{
			Duration: d,
			Label:    formatTimeframeLabel(d),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid timeframes")
	}
	return out, nil
}

// timeframeLabels collects the labels for log output.
func timeframeLabels(tfs []aggregator.Timeframe) []string {
	out := make([]string, len(tfs))
	for i, t := range tfs {
		out[i] = t.Label
	}
	return out
}

// formatTimeframeLabel renders the canonical short form ("1m", "5m",
// "1h"...) for use in subject/key names. Falls back to Go's default.
func formatTimeframeLabel(d time.Duration) string {
	switch d {
	case time.Minute:
		return "1m"
	case 5 * time.Minute:
		return "5m"
	case 15 * time.Minute:
		return "15m"
	case time.Hour:
		return "1h"
	case 4 * time.Hour:
		return "4h"
	case 24 * time.Hour:
		return "1d"
	default:
		return d.String()
	}
}

func defaultClientName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return "running-trade-consumer-" + host
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
