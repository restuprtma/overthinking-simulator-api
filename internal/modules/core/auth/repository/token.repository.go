package repository

import (
	"context"
	"encoding/json"
	"time"

	"venturo-skeleton-go/internal/modules/core/auth/domain"
	"venturo-skeleton-go/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRepository struct {
	db *pgxpool.Pool
}

func NewTokenRepository(db *pgxpool.Pool) *TokenRepository {
	return &TokenRepository{db: db}
}

// ===== Email Verification Tokens =====

func (r *TokenRepository) CreateEmailVerificationToken(userID, token string, expiresAt time.Time) error {
	ctx := context.Background()
	id := uuid.New().String()

	query := `
		INSERT INTO core.email_verification_tokens (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Exec(ctx, query, id, userID, token, expiresAt, time.Now())
	if err != nil {
		logger.Error("Failed to create email verification token", logger.Err(err))
		return err
	}

	return nil
}

func (r *TokenRepository) FindEmailVerificationToken(token string) (*domain.EmailVerificationToken, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, token, expires_at, verified_at, resent_count, last_resent_at, created_at
		FROM core.email_verification_tokens
		WHERE token = $1 AND deleted_at IS NULL
	`

	var evt domain.EmailVerificationToken
	err := r.db.QueryRow(ctx, query, token).Scan(
		&evt.ID, &evt.UserID, &evt.Token, &evt.ExpiresAt,
		&evt.VerifiedAt, &evt.ResentCount, &evt.LastResentAt, &evt.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &evt, nil
}

func (r *TokenRepository) MarkEmailAsVerified(token string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.email_verification_tokens
		SET verified_at = $1, updated_at = $2
		WHERE token = $3
	`

	_, err := r.db.Exec(ctx, query, now, now, token)
	return err
}

func (r *TokenRepository) IncrementEmailVerificationResend(userID string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.email_verification_tokens
		SET resent_count = resent_count + 1, last_resent_at = $1, updated_at = $2
		WHERE user_id = $3 AND verified_at IS NULL AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, now, now, userID)
	return err
}

func (r *TokenRepository) GetEmailVerificationResendCount(userID string) (int, error) {
	ctx := context.Background()

	query := `
		SELECT COALESCE(resent_count, 0)
		FROM core.email_verification_tokens
		WHERE user_id = $1 AND verified_at IS NULL AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ===== Password Reset Tokens =====

func (r *TokenRepository) CreatePasswordResetToken(userID, token string, expiresAt time.Time, ipAddress, userAgent *string) error {
	ctx := context.Background()
	id := uuid.New().String()

	query := `
		INSERT INTO core.password_reset_tokens (id, user_id, token, expires_at, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(ctx, query, id, userID, token, expiresAt, ipAddress, userAgent, time.Now())
	if err != nil {
		logger.Error("Failed to create password reset token", logger.Err(err))
		return err
	}

	return nil
}

func (r *TokenRepository) FindPasswordResetToken(token string) (*domain.PasswordResetToken, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, token, expires_at, used_at, ip_address, user_agent, created_at
		FROM core.password_reset_tokens
		WHERE token = $1
	`

	var prt domain.PasswordResetToken
	err := r.db.QueryRow(ctx, query, token).Scan(
		&prt.ID, &prt.UserID, &prt.Token, &prt.ExpiresAt,
		&prt.UsedAt, &prt.IPAddress, &prt.UserAgent, &prt.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &prt, nil
}

func (r *TokenRepository) MarkPasswordResetTokenAsUsed(token string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.password_reset_tokens
		SET used_at = $1
		WHERE token = $2
	`

	_, err := r.db.Exec(ctx, query, now, token)
	return err
}

func (r *TokenRepository) CountRecentPasswordResetRequests(userID string, duration time.Duration) (int, error) {
	ctx := context.Background()
	since := time.Now().Add(-duration)

	query := `
		SELECT COUNT(*)
		FROM core.password_reset_tokens
		WHERE user_id = $1 AND created_at > $2
	`

	var count int
	err := r.db.QueryRow(ctx, query, userID, since).Scan(&count)
	return count, err
}

// ===== Refresh Tokens =====

func (r *TokenRepository) CreateRefreshToken(rt *domain.RefreshToken) error {
	ctx := context.Background()

	query := `
		INSERT INTO core.refresh_tokens
		(id, user_id, token_hash, device_name, device_id, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(ctx, query,
		rt.ID, rt.UserID, rt.TokenHash, rt.DeviceName, rt.DeviceID,
		rt.IPAddress, rt.UserAgent, rt.ExpiresAt, rt.CreatedAt,
	)

	if err != nil {
		logger.Error("Failed to create refresh token", logger.Err(err))
		return err
	}

	return nil
}

func (r *TokenRepository) FindRefreshToken(tokenHash string) (*domain.RefreshToken, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, token_hash, device_name, device_id, ip_address, user_agent,
		       expires_at, last_used_at, revoked_at, created_at
		FROM core.refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL
	`

	var rt domain.RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.DeviceName, &rt.DeviceID,
		&rt.IPAddress, &rt.UserAgent, &rt.ExpiresAt, &rt.LastUsedAt,
		&rt.RevokedAt, &rt.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &rt, nil
}

func (r *TokenRepository) UpdateRefreshTokenLastUsed(tokenHash string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.refresh_tokens
		SET last_used_at = $1
		WHERE token_hash = $2
	`

	_, err := r.db.Exec(ctx, query, now, tokenHash)
	return err
}

func (r *TokenRepository) RevokeRefreshToken(tokenHash string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.refresh_tokens
		SET revoked_at = $1
		WHERE token_hash = $2
	`

	_, err := r.db.Exec(ctx, query, now, tokenHash)
	return err
}

func (r *TokenRepository) RevokeAllUserRefreshTokens(userID string) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.refresh_tokens
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, now, userID)
	return err
}

func (r *TokenRepository) GetUserRefreshTokens(userID string) ([]*domain.RefreshToken, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, token_hash, device_name, device_id, ip_address, user_agent,
		       expires_at, last_used_at, revoked_at, created_at
		FROM core.refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*domain.RefreshToken
	for rows.Next() {
		var rt domain.RefreshToken
		err := rows.Scan(
			&rt.ID, &rt.UserID, &rt.TokenHash, &rt.DeviceName, &rt.DeviceID,
			&rt.IPAddress, &rt.UserAgent, &rt.ExpiresAt, &rt.LastUsedAt,
			&rt.RevokedAt, &rt.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, &rt)
	}

	return tokens, nil
}

// ===== Login Attempts =====

func (r *TokenRepository) CreateLoginAttempt(la *domain.LoginAttempt) error {
	ctx := context.Background()

	query := `
		INSERT INTO core.login_attempts
		(id, user_id, email, username, ip_address, user_agent, status, failure_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(ctx, query,
		la.ID, la.UserID, la.Email, la.Username, la.IPAddress,
		la.UserAgent, la.Status, la.FailureReason, la.CreatedAt,
	)

	if err != nil {
		logger.Error("Failed to create login attempt", logger.Err(err))
		return err
	}

	return nil
}

func (r *TokenRepository) CountRecentFailedLoginAttempts(userID string, duration time.Duration) (int, error) {
	ctx := context.Background()
	since := time.Now().Add(-duration)

	query := `
		SELECT COUNT(*)
		FROM core.login_attempts
		WHERE user_id = $1 AND status = 'failed' AND created_at > $2
	`

	var count int
	err := r.db.QueryRow(ctx, query, userID, since).Scan(&count)
	return count, err
}

// ===== Audit Logs =====

func (r *TokenRepository) CreateAuditLog(al *domain.AuditLog) error {
	ctx := context.Background()

	var metadataJSON []byte
	var err error
	if al.Metadata != nil {
		metadataJSON, err = json.Marshal(al.Metadata)
		if err != nil {
			logger.Error("Failed to marshal audit log metadata", logger.Err(err))
			return err
		}
	}

	query := `
		INSERT INTO core.audit_logs
		(id, user_id, action, resource, resource_id, ip_address, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = r.db.Exec(ctx, query,
		al.ID, al.UserID, al.Action, al.Resource, al.ResourceID,
		al.IPAddress, al.UserAgent, metadataJSON, al.CreatedAt,
	)

	if err != nil {
		logger.Error("Failed to create audit log", logger.Err(err))
		return err
	}

	return nil
}
