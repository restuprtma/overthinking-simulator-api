package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"venturo-skeleton-go/internal/modules/reflection/domain"
	"venturo-skeleton-go/pkg/logger"
)

// ReflectionRepository owns reads/writes to core.reflections.
// Every query is scoped to a single user via user_id.
type ReflectionRepository struct {
	db *pgxpool.Pool
}

func NewReflectionRepository(db *pgxpool.Pool) *ReflectionRepository {
	return &ReflectionRepository{db: db}
}

// Create inserts a new reflection. DetectedDistortions and Dialog are
// marshaled to JSON and written to the JSONB columns.
func (r *ReflectionRepository) Create(ctx context.Context, ref *domain.Reflection) error {
	distortionsJSON, err := json.Marshal(ref.DetectedDistortions)
	if err != nil {
		logger.Error("Failed to marshal detected_distortions", logger.Err(err))
		return err
	}
	dialogJSON, err := json.Marshal(ref.Dialog)
	if err != nil {
		logger.Error("Failed to marshal dialog", logger.Err(err))
		return err
	}

	query := `
		INSERT INTO core.reflections (
			id, user_id, thought, detected_distortions, core_fear, dialog,
			actionable_suggestion, safety_triggered, safety_response, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.Exec(ctx, query,
		ref.ID, ref.UserID, ref.Thought, distortionsJSON, ref.CoreFear, dialogJSON,
		ref.ActionableSuggestion, ref.SafetyTriggered, ref.SafetyResponse, ref.CreatedAt,
	)
	if err != nil {
		logger.Error("Failed to create reflection", logger.Err(err))
		return err
	}
	return nil
}

// ListByUser returns reflections for a user, newest first, with pagination,
// plus the total count of that user's reflections.
func (r *ReflectionRepository) ListByUser(ctx context.Context, userID string, page, limit int) ([]domain.Reflection, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM core.reflections WHERE user_id = $1`, userID,
	).Scan(&total)
	if err != nil {
		logger.Error("Failed to count reflections", logger.Err(err))
		return nil, 0, err
	}

	offset := (page - 1) * limit
	query := `
		SELECT id, user_id, thought, detected_distortions, core_fear, dialog,
		       actionable_suggestion, safety_triggered, safety_response, created_at
		FROM core.reflections
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		logger.Error("Failed to list reflections", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	reflections := make([]domain.Reflection, 0)
	for rows.Next() {
		ref, err := scanReflection(rows)
		if err != nil {
			logger.Error("Failed to scan reflection", logger.Err(err))
			return nil, 0, err
		}
		reflections = append(reflections, *ref)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate reflections", logger.Err(err))
		return nil, 0, err
	}

	return reflections, total, nil
}

// GetByIDAndUser fetches a single reflection by id AND user_id.
// Returns (nil, nil) when no matching row exists.
func (r *ReflectionRepository) GetByIDAndUser(ctx context.Context, id, userID string) (*domain.Reflection, error) {
	query := `
		SELECT id, user_id, thought, detected_distortions, core_fear, dialog,
		       actionable_suggestion, safety_triggered, safety_response, created_at
		FROM core.reflections
		WHERE id = $1 AND user_id = $2
	`

	row := r.db.QueryRow(ctx, query, id, userID)
	ref, err := scanReflection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to get reflection", logger.Err(err))
		return nil, err
	}
	return ref, nil
}

// rowScanner is the minimal subset of pgx.Row / pgx.Rows needed to scan
// a single reflection row.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanReflection scans a reflection row and unmarshals the JSONB columns
// back into the domain slices.
func scanReflection(row rowScanner) (*domain.Reflection, error) {
	var ref domain.Reflection
	var distortionsJSON, dialogJSON []byte

	err := row.Scan(
		&ref.ID, &ref.UserID, &ref.Thought, &distortionsJSON, &ref.CoreFear, &dialogJSON,
		&ref.ActionableSuggestion, &ref.SafetyTriggered, &ref.SafetyResponse, &ref.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(distortionsJSON, &ref.DetectedDistortions); err != nil {
		logger.Error("Failed to unmarshal detected_distortions", logger.Err(err))
		return nil, err
	}
	if err := json.Unmarshal(dialogJSON, &ref.Dialog); err != nil {
		logger.Error("Failed to unmarshal dialog", logger.Err(err))
		return nil, err
	}

	return &ref, nil
}
