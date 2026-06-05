// Command iqplus-publisher reads the IQPlus TCP feed and republishes every
// record to NATS JetStream. Subjects + streams follow docs/infra/topology.md
// (idx.<kategori>.<id>) and route to four streams:
// IDX_TICK, IDX_QUOTE, IDX_META, IDX_NEWS.
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/iqplus-publisher-freebsd-amd64 ./cmd/iqplus-publisher
//
// Required environment variables:
//
//	IQPLUS_HOST       e.g. 103.114.143.237
//	IQPLUS_PORT       e.g. 8888
//	IQPLUS_USER
//	IQPLUS_PASS_MD5   md5(password) lowercase hex
//	NATS_URL          e.g. nats://10.10.8.2:4222
//	NATS_TOKEN        bearer token (optional)
//
// Optional:
//
//	NATS_CLIENT_NAME       default iqplus-publisher-<hostname>
//	NATS_PUBLISH_TIMEOUT   default 10s
//	NATS_ASYNC_MAX_PENDING default 32000   (PER stream — total = 4 * this)
//	NATS_DRAIN_TIMEOUT     default 30s
//	NATS_ENFORCE_STREAM    default true (set "false" if streams not yet created)
//	IQPLUS_DIAL_TIMEOUT    default 10s
//	IQPLUS_READ_TIMEOUT    default 60s
//	IQPLUS_RECONNECT_DELAY default 5s
//	IQPLUS_BUFFER_SIZE     default 2000000 (in-memory channel; sized for EOD resend burst)
//	IQPLUS_SEND_TIMEOUT    default 0       (0 = block forever; set e.g. 5s to drop after timeout)
//	IQPLUS_SOCKET_RECV_BUF default 16777216 (SO_RCVBUF bytes; capped by kern.ipc.maxsockbuf)
//	IQPLUS_BUFIO_READ_SIZE default 1048576 (bufio reader internal buffer bytes)
//	IQPLUS_TYPES_EXCLUDE   csv of record types to drop, e.g. "16,26,27"
//	STATS_INTERVAL         default 30s
//	ENV                    development|production (logger format)
//	DISCORD_ALERT_WEBHOOK  if set, warn+ log entries are tee'd to this Discord webhook
//	DISCORD_ALERT_LEVEL    warn|error (default warn — catches NATS ack errors)
//	DISCORD_ALERT_DEBOUNCE min interval between webhook posts (default 10s; bursts are batched)
//	DISCORD_ALERT_SOURCE   label prefixed onto messages (default $HOSTNAME)
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

	iqplus_publisher "tuai/internal/modules/stock/iqplus_publisher"
	"tuai/internal/modules/stock/iqplus_publisher/client"
	"tuai/internal/modules/stock/iqplus_publisher/publisher"
	"tuai/internal/modules/stock/iqplus_publisher/service"
	"tuai/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func init() {
	// Match cmd/api: force UTC across the binary.
	time.Local = time.UTC
}

func main() {
	// Load env file BEFORE logger init — ENV controls logger format.
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

	excluded := make([]int, 0, len(cfg.Service.ExcludeTypes))
	for t := range cfg.Service.ExcludeTypes {
		excluded = append(excluded, t)
	}

	logger.Log.Info("iqplus-publisher starting",
		zap.String("iqplus_host", cfg.Client.Host),
		zap.Int("iqplus_port", cfg.Client.Port),
		zap.String("iqplus_user", cfg.Client.User),
		zap.String("nats_url", cfg.Publisher.URL),
		zap.String("client_name", cfg.Publisher.ClientName),
		zap.Bool("enforce_stream", cfg.Publisher.EnforceStream),
		zap.Ints("excluded_types", excluded))

	mod := iqplus_publisher.Initialize(cfg)

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mod.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Log.Error("publisher exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("iqplus-publisher stopped cleanly")
}

func loadConfig() (iqplus_publisher.Config, error) {
	cli := client.DefaultConfig()
	cli.Host = mustEnv("IQPLUS_HOST")
	cli.User = mustEnv("IQPLUS_USER")
	cli.PasswordMD5 = mustEnv("IQPLUS_PASS_MD5")

	port, err := strconv.Atoi(mustEnv("IQPLUS_PORT"))
	if err != nil {
		return iqplus_publisher.Config{}, fmt.Errorf("IQPLUS_PORT invalid: %w", err)
	}
	cli.Port = port

	cli.DialTimeout = envDuration("IQPLUS_DIAL_TIMEOUT", cli.DialTimeout)
	cli.ReadTimeout = envDuration("IQPLUS_READ_TIMEOUT", cli.ReadTimeout)
	cli.ReconnectDelay = envDuration("IQPLUS_RECONNECT_DELAY", cli.ReconnectDelay)
	cli.BufferSize = envInt("IQPLUS_BUFFER_SIZE", cli.BufferSize)
	cli.SendTimeout = envDuration("IQPLUS_SEND_TIMEOUT", cli.SendTimeout)
	cli.SocketRecvBuffer = envInt("IQPLUS_SOCKET_RECV_BUF", cli.SocketRecvBuffer)
	cli.BufioReaderSize = envInt("IQPLUS_BUFIO_READ_SIZE", cli.BufioReaderSize)

	pub := publisher.DefaultConfig()
	pub.URL = mustEnv("NATS_URL")
	pub.Token = os.Getenv("NATS_TOKEN")
	pub.ClientName = envOr("NATS_CLIENT_NAME", defaultClientName())
	pub.PublishTimeout = envDuration("NATS_PUBLISH_TIMEOUT", pub.PublishTimeout)
	pub.AsyncMaxPending = envInt("NATS_ASYNC_MAX_PENDING", pub.AsyncMaxPending)
	pub.DrainOnShutdown = envDuration("NATS_DRAIN_TIMEOUT", pub.DrainOnShutdown)
	pub.SchemaVersion = envOr("PUBLISHER_SCHEMA_VERSION", pub.SchemaVersion)
	pub.EnforceStream = envBool("NATS_ENFORCE_STREAM", pub.EnforceStream)
	pub.SubjectPrefix = envOr("IQPLUS_SUBJECT_PREFIX", pub.SubjectPrefix)
	pub.RouteToStream = envOr("IQPLUS_ROUTE_TO_STREAM", pub.RouteToStream)

	svc := service.Config{
		StatsTick:    envDuration("STATS_INTERVAL", 30*time.Second),
		ExcludeTypes: parseTypeSet(os.Getenv("IQPLUS_TYPES_EXCLUDE")),
	}

	return iqplus_publisher.Config{
		Client:    cli,
		Publisher: pub,
		Service:   svc,
	}, nil
}

// loadEnvFile loads this service's .env file into the process environment.
//
// Per-service isolation: we deliberately do NOT fall back to the repo-root
// .env (which belongs to cmd/api) — each service owns its own config.
//
// Lookup order (first hit wins):
//  1. $IQPLUS_PUBLISHER_ENV_FILE       (explicit path — exits on error)
//  2. ./cmd/iqplus-publisher/.env      (repo root, e.g. via `go run`)
//  3. <exe-dir>/.env                   (binary deployed next to .env)
//  4. <exe-dir>/iqplus-publisher.env   (FreeBSD-style sibling config)
//
// godotenv.Load does NOT override existing process env, so values exported
// by systemd/rc.d/supervisor always win over the file.
func loadEnvFile() {
	if explicit := os.Getenv("IQPLUS_PUBLISHER_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load IQPLUS_PUBLISHER_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}

	candidates := []string{
		"cmd/iqplus-publisher/.env",
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "iqplus-publisher.env"),
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
	// No env file — process env must provide everything.
}

// parseTypeSet parses "16,26,27" into a set. Empty/blank entries are
// ignored; invalid integers log a warning and are skipped.
func parseTypeSet(csv string) map[int]struct{} {
	if csv == "" {
		return nil
	}
	out := make(map[int]struct{})
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			logger.Log.Warn("ignoring invalid record type in IQPLUS_TYPES_EXCLUDE",
				zap.String("value", p))
			continue
		}
		out[n] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func defaultClientName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return "iqplus-publisher-" + host
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
		logger.Log.Warn("invalid int env var, using default",
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
		logger.Log.Warn("invalid bool env var, using default",
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
		logger.Log.Warn("invalid duration env var, using default",
			zap.String("key", key), zap.String("value", v), zap.Duration("default", def))
		return def
	}
	return d
}
