package repository

import (
	"context"
	"errors"
	"fmt"

	"tuai/internal/modules/stock/daily_ingest/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository writes DailyBar rows to stock.prices_daily.
//
// Idempotency: ON CONFLICT (stock_code, date, market) DO UPDATE — safe to
// re-run for the same date (cron retry, manual backfill).
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// UpsertBars writes the given bars in a single transaction. Uses pgx batch
// to keep round-trips low even at ~900 rows per day. The function returns
// the number of rows that were INSERTED or UPDATED (which is len(bars) on
// success — Postgres doesn't easily distinguish the two for ON CONFLICT
// without RETURNING-and-counting, and we don't need the distinction here).
func (r *PostgresRepository) UpsertBars(
	ctx context.Context,
	bars []domain.DailyBar,
) (int, error) {
	if len(bars) == 0 {
		return 0, nil
	}

	const sql = `
INSERT INTO stock.prices_daily
    (stock_code, date, market, open, high, low, close, volume, value, freq)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (stock_code, date, market) DO UPDATE SET
    open   = EXCLUDED.open,
    high   = EXCLUDED.high,
    low    = EXCLUDED.low,
    close  = EXCLUDED.close,
    volume = EXCLUDED.volume,
    value  = EXCLUDED.value,
    freq   = EXCLUDED.freq`

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	batch := &pgx.Batch{}
	for _, b := range bars {
		batch.Queue(sql,
			b.StockCode, b.Date, b.Market,
			b.Open, b.High, b.Low, b.Close,
			b.Volume, b.Value, b.Freq,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := range bars {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return 0, fmt.Errorf("upsert row %d (stock=%s date=%s): %w",
				i, bars[i].StockCode, bars[i].Date.Format("2006-01-02"), err)
		}
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, err
		}
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(bars), nil
}
