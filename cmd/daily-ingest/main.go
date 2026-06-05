// Command daily-ingest aggregates QuestDB raw ticks into per-day OHLCV bars
// and upserts them into Postgres stock.prices_daily.
//
// Modes:
//
//	# Daemon (default): long-running, fires daily at DAILY_INGEST_RUN_AT (WIB)
//	./daily-ingest
//
//	# One-shot, default date = yesterday WIB
//	./daily-ingest --once
//
//	# One-shot, specific date
//	./daily-ingest --once --date=2026-04-25
//
//	# One-shot, date range (inclusive)
//	./daily-ingest --once --from=2026-04-01 --to=2026-04-25
//
// Build for FreeBSD:
//
//	GOOS=freebsd GOARCH=amd64 go build -o bin/daily-ingest-freebsd-amd64 ./cmd/daily-ingest
//
// Required env vars:
//
//	DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
//	QUESTDB_HTTP_URL          e.g. http://localhost:9000
//
// Optional env vars (with defaults):
//
//	DB_SSLMODE                disable
//	QUESTDB_TABLE             trades
//	QUESTDB_TS_COLUMN         timestamp     (verify with SHOW COLUMNS FROM trades)
//	QUESTDB_AUTH_USER         (basic auth alternative)
//	QUESTDB_AUTH_PASSWORD
//	QUESTDB_BEARER_TOKEN
//	QUESTDB_HTTP_TIMEOUT      60s
//
//	DAILY_INGEST_DEFAULT_MARKET  RG         (fallback for legacy NULL market rows;
//	                                         live data is read per-row from QuestDB)
//	DAILY_INGEST_RUN_AT          17:00      (HH:MM in Asia/Jakarta)
//	DAILY_INGEST_TARGET          today      (today | yesterday)
//
//	ENV                       production    (development = console logger)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tuai/internal/database"
	dailyingest "tuai/internal/modules/stock/daily_ingest"
	"tuai/internal/modules/stock/daily_ingest/repository"
	"tuai/internal/modules/stock/daily_ingest/service"
	"tuai/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

var jakartaLoc = time.FixedZone("WIB", 7*3600)

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

	flags := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		logger.Log.Fatal("config error", zap.Error(err))
	}

	db, err := buildPgPool(ctx)
	if err != nil {
		logger.Log.Fatal("postgres pool", zap.Error(err))
	}
	defer db.Close()

	mod, err := dailyingest.Initialize(ctx, cfg, db.Pool)
	if err != nil {
		logger.Log.Fatal("module init", zap.Error(err))
	}

	if flags.once {
		if err := runOnce(ctx, mod.Service, flags); err != nil &&
			!errors.Is(err, context.Canceled) {
			logger.Log.Error("one-shot ingest failed", zap.Error(err))
			os.Exit(1)
		}
		logger.Log.Info("daily-ingest one-shot completed cleanly")
		return
	}

	logger.Log.Info("daily-ingest daemon starting",
		zap.String("default_market", cfg.Service.DefaultMarket),
		zap.Duration("run_at", cfg.Service.RunAt),
		zap.String("target", cfg.Service.TargetMode))

	if err := mod.Service.Run(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		logger.Log.Error("daemon exited with error", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("daily-ingest daemon stopped cleanly")
}

// flagsT is the parsed CLI surface.
type flagsT struct {
	once bool
	date string // "YYYY-MM-DD"
	from string
	to   string
}

func parseFlags() flagsT {
	fs := flag.NewFlagSet("daily-ingest", flag.ExitOnError)
	once := fs.Bool("once", false,
		"run a single ingest and exit (default: daemon mode)")
	date := fs.String("date", "",
		"WIB date YYYY-MM-DD for --once (default: yesterday WIB)")
	from := fs.String("from", "",
		"WIB start date YYYY-MM-DD for --once range")
	to := fs.String("to", "",
		"WIB end date YYYY-MM-DD for --once range (inclusive)")

	_ = fs.Parse(os.Args[1:])

	return flagsT{
		once: *once,
		date: *date,
		from: *from,
		to:   *to,
	}
}

func runOnce(ctx context.Context, svc *service.Service, f flagsT) error {
	switch {
	case f.from != "" || f.to != "":
		if f.from == "" || f.to == "" {
			return errors.New("--from and --to must be used together")
		}
		if f.date != "" {
			return errors.New("--date conflicts with --from/--to")
		}
		from, err := parseDate(f.from)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		to, err := parseDate(f.to)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		_, err = svc.IngestRange(ctx, from, to)
		return err

	case f.date != "":
		day, err := parseDate(f.date)
		if err != nil {
			return fmt.Errorf("--date: %w", err)
		}
		_, err = svc.IngestDate(ctx, day)
		return err

	default:
		// No date specified — default to yesterday WIB. This matches the
		// most common cron pattern "ingest yesterday after midnight".
		yesterdayWIB := time.Now().In(jakartaLoc).AddDate(0, 0, -1)
		day := time.Date(
			yesterdayWIB.Year(), yesterdayWIB.Month(), yesterdayWIB.Day(),
			0, 0, 0, 0, time.UTC,
		)
		logger.Log.Info("--once invoked without --date — defaulting to yesterday WIB",
			zap.String("date", day.Format("2006-01-02")))
		_, err := svc.IngestDate(ctx, day)
		return err
	}
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD, got %q", s)
	}
	return t, nil
}

func loadConfig() (dailyingest.Config, error) {
	qdb := repository.DefaultQuestDBConfig()
	qdb.BaseURL = mustEnv("QUESTDB_HTTP_URL")
	qdb.Table = envOr("QUESTDB_TABLE", qdb.Table)
	qdb.TsColumn = envOr("QUESTDB_TS_COLUMN", qdb.TsColumn)
	qdb.BearerToken = os.Getenv("QUESTDB_BEARER_TOKEN")
	qdb.BasicAuthUser = os.Getenv("QUESTDB_AUTH_USER")
	qdb.BasicAuthPassword = os.Getenv("QUESTDB_AUTH_PASSWORD")
	qdb.HTTPTimout = envDuration("QUESTDB_HTTP_TIMEOUT", qdb.HTTPTimout)

	runAt, err := parseRunAt(envOr("DAILY_INGEST_RUN_AT", "17:00"))
	if err != nil {
		return dailyingest.Config{}, fmt.Errorf("DAILY_INGEST_RUN_AT: %w", err)
	}

	target := strings.ToLower(envOr("DAILY_INGEST_TARGET", "today"))
	if target != "today" && target != "yesterday" {
		return dailyingest.Config{},
			fmt.Errorf("DAILY_INGEST_TARGET must be 'today' or 'yesterday', got %q", target)
	}

	svcCfg := service.Config{
		DefaultMarket: envOr("DAILY_INGEST_DEFAULT_MARKET", "RG"),
		RunAt:         runAt,
		TargetMode:    target,
	}

	return dailyingest.Config{
		QuestDB: qdb,
		Service: svcCfg,
	}, nil
}

// parseRunAt accepts "HH:MM" and returns the offset from WIB midnight.
func parseRunAt(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	h, err := time.ParseDuration(parts[0] + "h")
	if err != nil {
		return 0, fmt.Errorf("hours: %w", err)
	}
	m, err := time.ParseDuration(parts[1] + "m")
	if err != nil {
		return 0, fmt.Errorf("minutes: %w", err)
	}
	d := h + m
	if d < 0 || d >= 24*time.Hour {
		return 0, fmt.Errorf("must be 00:00..23:59, got %s", d)
	}
	return d, nil
}

// buildPgPool reuses the standard tuai/internal/database wiring so this
// service honours the same DB_MAX_CONNS / lifetime / health-check defaults
// as the rest of the codebase. The returned *Database owns Close().
func buildPgPool(ctx context.Context) (*database.Database, error) {
	dsn := postgresDSN()
	db, err := database.New(dsn)
	if err != nil {
		return nil, err
	}
	// Belt-and-braces ping (database.New already pings, but it uses its
	// own background context — re-verify under the parent ctx).
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.Pool.Ping(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return db, nil
}

// loadEnvFile mirrors the per-service convention used by running-trade-consumer.
//
// Lookup order:
//  1. $DAILY_INGEST_ENV_FILE         (explicit)
//  2. ./cmd/daily-ingest/.env        (repo root via `go run`)
//  3. <exe-dir>/.env
//  4. <exe-dir>/daily-ingest.env
func loadEnvFile() {
	if explicit := os.Getenv("DAILY_INGEST_ENV_FILE"); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			fmt.Fprintf(os.Stderr,
				"failed to load DAILY_INGEST_ENV_FILE=%s: %v\n",
				explicit, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loaded env from %s\n", explicit)
		return
	}
	candidates := []string{"cmd/daily-ingest/.env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "daily-ingest.env"),
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

func postgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&timezone=UTC",
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", "postgres"),
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_NAME", "tuai"),
		envOr("DB_SSLMODE", "disable"),
	)
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
