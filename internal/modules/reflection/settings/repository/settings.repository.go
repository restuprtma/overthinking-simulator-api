package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"venturo-skeleton-go/pkg/logger"
)

type SettingsRepository struct {
	db *pgxpool.Pool
}

func NewSettingsRepository(db *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get returns the value for the given key. If no row exists it returns
// an empty string and a nil error.
func (r *SettingsRepository) Get(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM core.settings WHERE key = $1`

	var value string
	err := r.db.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		logger.Error("Failed to get setting", logger.String("key", key), logger.Err(err))
		return "", err
	}

	return value, nil
}

// Set upserts the value for the given key.
func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	query := `
		INSERT INTO core.settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`

	_, err := r.db.Exec(ctx, query, key, value)
	if err != nil {
		logger.Error("Failed to set setting", logger.String("key", key), logger.Err(err))
		return err
	}

	return nil
}
