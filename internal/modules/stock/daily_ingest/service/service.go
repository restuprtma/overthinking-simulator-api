// Package service orchestrates one full ingest cycle: query QuestDB for
// per-day OHLCV → UPSERT into Postgres stock.prices_daily.
//
// Two modes are exposed:
//
//   - IngestDate / IngestRange: one-shot calls used by --once CLI runs and
//     by the daemon's daily tick.
//   - Run: long-running daemon that fires IngestDate once a day at the
//     configured WIB clock time.
package service

import (
	"context"
	"fmt"
	"time"

	"tuai/internal/modules/stock/daily_ingest/domain"
	"tuai/pkg/logger"

	"go.uber.org/zap"
)

// Config holds runtime knobs for both daemon and one-shot modes.
//
// RunAt is interpreted in Asia/Jakarta — the IDX trading wall clock —
// because that's how operators reason about "EOD". TargetMode picks which
// trading day the daemon ingests when its scheduled tick fires.
//
// DefaultMarket is the fallback IDX board code used only when the QuestDB
// `market` column is NULL on a row (legacy data from before the column was
// added). Live data has market populated by the consumer that wrote it.
type Config struct {
	DefaultMarket string        // fallback for NULL market in legacy rows (default "RG")
	RunAt         time.Duration // daemon fire offset from WIB midnight (default 17h)
	TargetMode    string        // "yesterday" or "today" (default "today")
}

// QuestDBRepo is the read-side dependency.
type QuestDBRepo interface {
	FetchDailyBars(ctx context.Context, startWIB, endWIBExclusive time.Time, market string) ([]domain.DailyBar, error)
}

// PostgresRepo is the write-side dependency.
type PostgresRepo interface {
	UpsertBars(ctx context.Context, bars []domain.DailyBar) (int, error)
}

type Service struct {
	cfg Config
	qdb QuestDBRepo
	pg  PostgresRepo
}

func New(cfg Config, qdb QuestDBRepo, pg PostgresRepo) *Service {
	if cfg.DefaultMarket == "" {
		cfg.DefaultMarket = "RG"
	}
	if cfg.RunAt <= 0 {
		cfg.RunAt = 17 * time.Hour
	}
	if cfg.TargetMode == "" {
		cfg.TargetMode = "today"
	}
	return &Service{cfg: cfg, qdb: qdb, pg: pg}
}

// IngestDate aggregates and upserts one trading day (WIB).
func (s *Service) IngestDate(ctx context.Context, day time.Time) (int, error) {
	startWIB, endWIB := dayBoundsWIB(day)
	t0 := time.Now()

	bars, err := s.qdb.FetchDailyBars(ctx, startWIB, endWIB, s.cfg.DefaultMarket)
	if err != nil {
		return 0, fmt.Errorf("questdb fetch %s: %w",
			day.Format("2006-01-02"), err)
	}
	if len(bars) == 0 {
		logger.Log.Info("daily-ingest: no trades for date — skipping upsert",
			zap.String("date", day.Format("2006-01-02")),
			zap.String("default_market", s.cfg.DefaultMarket))
		return 0, nil
	}

	written, err := s.pg.UpsertBars(ctx, bars)
	if err != nil {
		return 0, fmt.Errorf("postgres upsert %s: %w",
			day.Format("2006-01-02"), err)
	}

	logger.Log.Info("daily-ingest: ingested",
		zap.String("date", day.Format("2006-01-02")),
		zap.Int("rows", written),
		zap.Duration("elapsed", time.Since(t0).Round(time.Millisecond)))
	return written, nil
}

// IngestRange runs IngestDate for each day in [from, to] inclusive. Stops
// at the first error — caller decides whether to retry.
func (s *Service) IngestRange(ctx context.Context, from, to time.Time) (int, error) {
	if to.Before(from) {
		return 0, fmt.Errorf("--to %s before --from %s",
			to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	total := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		n, err := s.IngestDate(ctx, d)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// Run blocks until ctx is done, firing IngestDate once a day at RunAt
// (WIB). Catches up if the binary started past today's window: ingests
// immediately, then sleeps until tomorrow's RunAt.
func (s *Service) Run(ctx context.Context) error {
	for {
		next := nextFireTime(time.Now(), s.cfg.RunAt)
		wait := time.Until(next)

		logger.Log.Info("daily-ingest daemon scheduled",
			zap.Time("next_fire_utc", next.UTC()),
			zap.Time("next_fire_wib", next.In(jakartaLoc)),
			zap.Duration("sleep", wait.Round(time.Second)))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		target := s.targetDate(time.Now())
		if _, err := s.IngestDate(ctx, target); err != nil {
			// Don't kill the daemon on a single-day failure — log and try
			// again tomorrow. Operators can manually rerun via --once.
			logger.Log.Error("daily-ingest tick failed",
				zap.String("date", target.Format("2006-01-02")),
				zap.Error(err))
		}
	}
}

// targetDate decides which trading day to ingest given the current time.
//
// "today" mode: ingest the calendar date the daemon woke up on. Suitable
// when RunAt is post-close (default 17:00 WIB) — all trades for today
// are already in QuestDB.
//
// "yesterday" mode: ingest the previous WIB day. Suitable for early-AM
// runs (e.g. RunAt = 02:00 WIB tomorrow morning).
func (s *Service) targetDate(now time.Time) time.Time {
	wib := now.In(jakartaLoc)
	day := time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, time.UTC)
	if s.cfg.TargetMode == "yesterday" {
		day = day.AddDate(0, 0, -1)
	}
	return day
}

// nextFireTime returns the next absolute time at which the daemon should
// run IngestDate. If today's slot has already passed, it returns
// tomorrow's; otherwise today's.
func nextFireTime(now time.Time, runAt time.Duration) time.Time {
	wib := now.In(jakartaLoc)
	todayMidnightWIB := time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, jakartaLoc)
	candidate := todayMidnightWIB.Add(runAt)
	if !candidate.After(wib) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

// dayBoundsWIB returns the [start, endExclusive) bounds for one WIB
// calendar day, expressed in WIB (caller converts to UTC for the QuestDB
// query). day's H/M/S fields are ignored.
func dayBoundsWIB(day time.Time) (time.Time, time.Time) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, jakartaLoc)
	return start, start.AddDate(0, 0, 1)
}

var jakartaLoc = time.FixedZone("WIB", 7*3600)
