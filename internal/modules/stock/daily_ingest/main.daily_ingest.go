// Package daily_ingest wires the QuestDB read repository, Postgres write
// repository, and orchestration service for the EOD daily OHLCV ingest.
//
// Source of truth: QuestDB `trades` (raw ticks) → SAMPLE BY 1d aligned to
// Asia/Jakarta → UPSERT into Postgres stock.prices_daily.
//
// Multi-board (RG/TN/NG): the QuestDB schema currently has no `board`
// column, so this module hard-codes a single market value (configured via
// Service.Market, default "RG"). When the trades schema gains a board
// column, FetchDailyBars should split per-board in a single query.
package daily_ingest

import (
	"context"
	"fmt"

	"tuai/internal/modules/stock/daily_ingest/repository"
	"tuai/internal/modules/stock/daily_ingest/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Module bundles the long-lived components needed for either daemon or
// one-shot mode.
type Module struct {
	QuestDB  *repository.QuestDBRepository
	Postgres *repository.PostgresRepository
	Service  *service.Service
}

// Config is the union of all sub-configs.
type Config struct {
	QuestDB repository.QuestDBConfig
	Service service.Config
}

// Initialize builds the module against an already-connected pgx pool. The
// pool is owned by the caller (cmd/daily-ingest) so it can be closed on
// shutdown.
func Initialize(ctx context.Context, cfg Config, pool *pgxpool.Pool) (*Module, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool must be non-nil")
	}
	qdb := repository.NewQuestDB(cfg.QuestDB)
	pg := repository.NewPostgres(pool)
	svc := service.New(cfg.Service, qdb, pg)
	return &Module{
		QuestDB:  qdb,
		Postgres: pg,
		Service:  svc,
	}, nil
}
