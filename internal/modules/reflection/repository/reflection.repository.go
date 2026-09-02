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

// ErrDialogLimitReached signals that a reflection's dialog already holds the
// maximum number of turns, so no further turns were appended.
var ErrDialogLimitReached = errors.New("dialog turn limit reached")

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

	if ref.ConversationState == "" {
		ref.ConversationState = domain.ConversationInitial
	}
	ref.TotalTurns = len(ref.Dialog)

	query := `
		INSERT INTO core.reflections (
			id, user_id, thought, detected_distortions, core_fear, dialog,
			actionable_suggestion, safety_triggered, safety_response,
			conversation_state, total_turns, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = r.db.Exec(ctx, query,
		ref.ID, ref.UserID, ref.Thought, distortionsJSON, ref.CoreFear, dialogJSON,
		ref.ActionableSuggestion, ref.SafetyTriggered, ref.SafetyResponse,
		ref.ConversationState, ref.TotalTurns, ref.CreatedAt,
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
		       actionable_suggestion, safety_triggered, safety_response,
		       conversation_state, total_turns, created_at
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
		       actionable_suggestion, safety_triggered, safety_response,
		       conversation_state, total_turns, created_at
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

// AppendDialogTurns atomically appends turns to an existing reflection's dialog
// and returns the resulting state. The concatenation happens inside the UPDATE
// (`dialog || $1::jsonb`) rather than read-modify-write in Go, so two concurrent
// continuations cannot overwrite each other's turns.
//
// maxTurns bounds the growth: when the stored dialog already has that many turns
// the update is a no-op and ErrDialogLimitReached is returned.
func (r *ReflectionRepository) AppendDialogTurns(ctx context.Context, id, userID string, turns []domain.DialogTurn, maxTurns int) (*domain.DialogState, error) {
	turnsJSON, err := json.Marshal(turns)
	if err != nil {
		logger.Error("Failed to marshal dialog turns", logger.Err(err))
		return nil, err
	}

	query := `
		UPDATE core.reflections
		SET dialog = dialog || $1::jsonb,
		    total_turns = jsonb_array_length(dialog || $1::jsonb),
		    conversation_state = CASE
		        WHEN jsonb_array_length(dialog || $1::jsonb) >= $4 THEN 'final'
		        ELSE 'continued'
		    END
		WHERE id = $2 AND user_id = $3 AND jsonb_array_length(dialog) < $4
		RETURNING dialog, conversation_state, total_turns
	`

	var dialogJSON []byte
	state := &domain.DialogState{}

	err = r.db.QueryRow(ctx, query, turnsJSON, id, userID, maxTurns).
		Scan(&dialogJSON, &state.ConversationState, &state.TotalTurns)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either the row is gone or the dialog already hit the cap. Tell the
			// two cases apart so the caller can map them to different statuses.
			var turnCount int
			checkErr := r.db.QueryRow(ctx,
				`SELECT jsonb_array_length(dialog) FROM core.reflections WHERE id = $1 AND user_id = $2`,
				id, userID,
			).Scan(&turnCount)
			if checkErr != nil {
				if errors.Is(checkErr, pgx.ErrNoRows) {
					return nil, pgx.ErrNoRows
				}
				logger.Error("Failed to inspect dialog length", logger.Err(checkErr))
				return nil, checkErr
			}
			return nil, ErrDialogLimitReached
		}
		logger.Error("Failed to append dialog turns", logger.Err(err))
		return nil, err
	}

	if err := json.Unmarshal(dialogJSON, &state.Dialog); err != nil {
		logger.Error("Failed to unmarshal appended dialog", logger.Err(err))
		return nil, err
	}

	return state, nil
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
		&ref.ActionableSuggestion, &ref.SafetyTriggered, &ref.SafetyResponse,
		&ref.ConversationState, &ref.TotalTurns, &ref.CreatedAt,
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
