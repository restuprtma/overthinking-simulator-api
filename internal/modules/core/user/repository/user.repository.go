package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lakukan-be/internal/modules/core/user/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(email string) (*domain.User, error) {
	ctx := context.Background()
	query := `
		SELECT u.id, u.email, u.username, u.password_hash, u.full_name, u.phone, u.is_active,
		       u.is_email_verified, u.last_login_at, u.created_at, COALESCE(u1.full_name, u.created_by::TEXT) as created_by,
		       u.updated_at, COALESCE(u2.full_name, u.updated_by::TEXT) as updated_by,
		       u.deleted_at, COALESCE(u3.full_name, u.deleted_by::TEXT) as deleted_by
		FROM core.users u
		LEFT JOIN core.users u1 ON u.created_by = u1.id
		LEFT JOIN core.users u2 ON u.updated_by = u2.id
		LEFT JOIN core.users u3 ON u.deleted_by = u3.id
		WHERE u.email = $1 AND u.deleted_at IS NULL
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// FindByUsername finds a user by username
func (r *UserRepository) FindByUsername(username string) (*domain.User, error) {
	ctx := context.Background()
	query := `
		SELECT u.id, u.email, u.username, u.password_hash, u.full_name, u.phone, u.is_active,
		       u.is_email_verified, u.last_login_at, u.created_at, COALESCE(u1.full_name, u.created_by::TEXT) as created_by,
		       u.updated_at, COALESCE(u2.full_name, u.updated_by::TEXT) as updated_by,
		       u.deleted_at, COALESCE(u3.full_name, u.deleted_by::TEXT) as deleted_by
		FROM core.users u
		LEFT JOIN core.users u1 ON u.created_by = u1.id
		LEFT JOIN core.users u2 ON u.updated_by = u2.id
		LEFT JOIN core.users u3 ON u.deleted_by = u3.id
		WHERE u.username = $1 AND u.deleted_at IS NULL
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(id string) (*domain.User, error) {
	ctx := context.Background()
	query := `
		SELECT u.id, u.email, u.username, u.password_hash, u.full_name, u.phone, u.is_active,
		       u.is_email_verified, u.last_login_at, u.created_at, COALESCE(u1.full_name, u.created_by::TEXT) as created_by,
		       u.updated_at, COALESCE(u2.full_name, u.updated_by::TEXT) as updated_by,
		       u.deleted_at, COALESCE(u3.full_name, u.deleted_by::TEXT) as deleted_by
		FROM core.users u
		LEFT JOIN core.users u1 ON u.created_by = u1.id
		LEFT JOIN core.users u2 ON u.updated_by = u2.id
		LEFT JOIN core.users u3 ON u.deleted_by = u3.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(userID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.users
		SET last_login_at = $1, updated_at = $1
		WHERE id = $2
	`

	_, err := r.db.Exec(ctx, query, time.Now(), userID)
	return err
}

// UpdateEmailVerified updates the email verified status
func (r *UserRepository) UpdateEmailVerified(userID string, verified bool) error {
	ctx := context.Background()
	now := time.Now()

	query := `
		UPDATE core.users
		SET is_email_verified = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(ctx, query, verified, now, userID)
	return err
}

// Create creates a new user
func (r *UserRepository) Create(user *domain.User) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.users (
			id, email, username, password_hash, full_name, phone,
			is_active, is_email_verified, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Exec(ctx, query,
		user.ID, user.Email, user.Username, user.PasswordHash,
		user.FullName, user.Phone, user.IsActive, user.IsEmailVerified,
		user.CreatedAt, user.CreatedBy, user.UpdatedAt, user.UpdatedBy,
	)

	return err
}

// Update updates a user
func (r *UserRepository) Update(user *domain.User) error {
	ctx := context.Background()
	query := `
		UPDATE core.users
		SET email = $1, username = $2, full_name = $3, phone = $4,
		    is_active = $5, updated_at = $6, updated_by = $7
		WHERE id = $8 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		user.Email, user.Username, user.FullName, user.Phone,
		user.IsActive, user.UpdatedAt, user.UpdatedBy, user.ID,
	)

	return err
}

// UpdatePassword updates user password
func (r *UserRepository) UpdatePassword(userID string, passwordHash string, updatedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.users
		SET password_hash = $1, updated_at = $2, updated_by = $3
		WHERE id = $4 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, passwordHash, time.Now(), updatedBy, userID)
	return err
}

// Delete soft deletes a user
func (r *UserRepository) Delete(userID string, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.users
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), deletedBy, userID)
	return err
}

// Restore restores a soft-deleted user
func (r *UserRepository) Restore(userID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.users
		SET deleted_at = NULL, deleted_by = NULL, updated_at = $1
		WHERE id = $2 AND deleted_at IS NOT NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), userID)
	return err
}

// UserWithRoles represents a user with their associated roles
type UserWithRoles struct {
	User  domain.User
	Roles []RoleInfo
}

// RoleInfo represents role information
type RoleInfo struct {
	ID          string
	Name        string
	Description *string
	IsSystem    bool
}

// FindAll finds all users with pagination and filters
func (r *UserRepository) FindAll(limit, offset int, search, fullName, username, email string, isActive *bool) ([]domain.User, error) {
	ctx := context.Background()
	query := `
		SELECT u.id, u.email, u.username, u.password_hash, u.full_name, u.phone, u.is_active,
		       u.is_email_verified, u.last_login_at, u.created_at, COALESCE(u1.full_name, u.created_by::TEXT) as created_by,
		       u.updated_at, COALESCE(u2.full_name, u.updated_by::TEXT) as updated_by,
		       u.deleted_at, COALESCE(u3.full_name, u.deleted_by::TEXT) as deleted_by
		FROM core.users u
		LEFT JOIN core.users u1 ON u.created_by = u1.id
		LEFT JOIN core.users u2 ON u.updated_by = u2.id
		LEFT JOIN core.users u3 ON u.deleted_by = u3.id
		WHERE u.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter (OR condition across multiple fields)
	if search != "" {
		query += fmt.Sprintf(` AND (u.email ILIKE $%d OR u.username ILIKE $%d OR u.full_name ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add specific field filters (AND conditions)
	if fullName != "" {
		query += fmt.Sprintf(` AND u.full_name ILIKE $%d`, argPos)
		args = append(args, "%"+fullName+"%")
		argPos++
	}

	if username != "" {
		query += fmt.Sprintf(` AND u.username ILIKE $%d`, argPos)
		args = append(args, "%"+username+"%")
		argPos++
	}

	if email != "" {
		query += fmt.Sprintf(` AND u.email ILIKE $%d`, argPos)
		args = append(args, "%"+email+"%")
		argPos++
	}

	// Add isActive filter
	if isActive != nil {
		query += fmt.Sprintf(` AND u.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	query += fmt.Sprintf(` ORDER BY u.created_at DESC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var user domain.User
		err := rows.Scan(
			&user.ID, &user.Email, &user.Username, &user.PasswordHash,
			&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
			&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
			&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// FindAllWithRoles finds all users with their roles
func (r *UserRepository) FindAllWithRoles(limit, offset int, search, fullName, username, email string, isActive *bool) ([]UserWithRoles, error) {
	ctx := context.Background()

	// First, get the users
	users, err := r.FindAll(limit, offset, search, fullName, username, email, isActive)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []UserWithRoles{}, nil
	}

	// Collect user IDs
	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}

	// Fetch roles for all users in a single query
	rolesQuery := `
		SELECT
			ur.user_id,
			r.id,
			r.name,
			r.description,
			r.is_system
		FROM core.user_roles ur
		INNER JOIN core.roles r ON ur.role_id = r.id
		WHERE ur.user_id = ANY($1)
		  AND ur.deleted_at IS NULL
		  AND r.deleted_at IS NULL
		ORDER BY r.name
	`

	rows, err := r.db.Query(ctx, rolesQuery, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map user ID to their roles
	userRolesMap := make(map[string][]RoleInfo)
	for rows.Next() {
		var userID string
		var role RoleInfo

		err := rows.Scan(
			&userID,
			&role.ID,
			&role.Name,
			&role.Description,
			&role.IsSystem,
		)
		if err != nil {
			return nil, err
		}

		userRolesMap[userID] = append(userRolesMap[userID], role)
	}

	// Combine users with their roles
	result := make([]UserWithRoles, len(users))
	for i, user := range users {
		result[i] = UserWithRoles{
			User:  user,
			Roles: userRolesMap[user.ID],
		}
		// Initialize empty slice if user has no roles
		if result[i].Roles == nil {
			result[i].Roles = []RoleInfo{}
		}
	}

	return result, nil
}

// Count counts total users with filters
func (r *UserRepository) Count(search, fullName, username, email string, isActive *bool) (int64, error) {
	ctx := context.Background()
	query := `SELECT COUNT(*) FROM core.users u WHERE u.deleted_at IS NULL`

	args := []interface{}{}
	argPos := 1

	// Add search filter (OR condition across multiple fields)
	if search != "" {
		query += fmt.Sprintf(` AND (u.email ILIKE $%d OR u.username ILIKE $%d OR u.full_name ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add specific field filters (AND conditions)
	if fullName != "" {
		query += fmt.Sprintf(` AND u.full_name ILIKE $%d`, argPos)
		args = append(args, "%"+fullName+"%")
		argPos++
	}

	if username != "" {
		query += fmt.Sprintf(` AND u.username ILIKE $%d`, argPos)
		args = append(args, "%"+username+"%")
		argPos++
	}

	if email != "" {
		query += fmt.Sprintf(` AND u.email ILIKE $%d`, argPos)
		args = append(args, "%"+email+"%")
		argPos++
	}

	// Add isActive filter
	if isActive != nil {
		query += fmt.Sprintf(` AND u.is_active = $%d`, argPos)
		args = append(args, *isActive)
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// FindByIDWithRoles finds a user by ID and includes their roles and permissions
func (r *UserRepository) FindByIDWithRoles(id string) (*domain.User, []string, []string, error) {
	ctx := context.Background()
	query := `
		SELECT
			u.id, u.email, u.username, u.password_hash, u.full_name, u.phone,
			u.is_active, u.is_email_verified, u.last_login_at, u.created_at,
			u.created_by, u.updated_at, u.updated_by, u.deleted_at, u.deleted_by,
			COALESCE(
				json_agg(DISTINCT r.name) FILTER (WHERE r.name IS NOT NULL),
				'[]'
			) as roles,
			COALESCE(
				json_agg(DISTINCT (p.resource || ':' || p.action)) FILTER (WHERE p.resource IS NOT NULL),
				'[]'
			) as permissions
		FROM core.users u
		LEFT JOIN core.user_roles ur ON u.id = ur.user_id AND ur.deleted_at IS NULL
		LEFT JOIN core.roles r ON ur.role_id = r.id AND r.deleted_at IS NULL
		LEFT JOIN core.role_permissions rp ON r.id = rp.role_id AND rp.deleted_at IS NULL
		LEFT JOIN core.permissions p ON rp.permission_id = p.id AND p.deleted_at IS NULL
		WHERE u.id = $1 AND u.deleted_at IS NULL
		GROUP BY u.id
	`

	var user domain.User
	var rolesJSON, permissionsJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
		&rolesJSON, &permissionsJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, nil // User not found
		}
		return nil, nil, nil, err
	}

	// Parse roles JSON
	var roles []string
	if len(rolesJSON) > 0 && string(rolesJSON) != "[]" {
		// Simple JSON array parsing (e.g., ["admin","user"])
		rolesStr := string(rolesJSON)
		rolesStr = rolesStr[1 : len(rolesStr)-1] // Remove [ ]
		if rolesStr != "" {
			// Split by comma and clean quotes
			parts := splitAndClean(rolesStr)
			roles = parts
		}
	}

	// Parse permissions JSON
	var permissions []string
	if len(permissionsJSON) > 0 && string(permissionsJSON) != "[]" {
		permStr := string(permissionsJSON)
		permStr = permStr[1 : len(permStr)-1] // Remove [ ]
		if permStr != "" {
			parts := splitAndClean(permStr)
			permissions = parts
		}
	}

	return &user, roles, permissions, nil
}

// FindByEmailWithRoles finds a user by email and includes their roles and permissions
func (r *UserRepository) FindByEmailWithRoles(email string) (*domain.User, []string, []string, error) {
	ctx := context.Background()
	query := `
		SELECT
			u.id, u.email, u.username, u.password_hash, u.full_name, u.phone,
			u.is_active, u.is_email_verified, u.last_login_at, u.created_at,
			u.created_by, u.updated_at, u.updated_by, u.deleted_at, u.deleted_by,
			COALESCE(
				json_agg(DISTINCT r.name) FILTER (WHERE r.name IS NOT NULL),
				'[]'
			) as roles,
			COALESCE(
				json_agg(DISTINCT (p.resource || ':' || p.action)) FILTER (WHERE p.resource IS NOT NULL),
				'[]'
			) as permissions
		FROM core.users u
		LEFT JOIN core.user_roles ur ON u.id = ur.user_id AND ur.deleted_at IS NULL
		LEFT JOIN core.roles r ON ur.role_id = r.id AND r.deleted_at IS NULL
		LEFT JOIN core.role_permissions rp ON r.id = rp.role_id AND rp.deleted_at IS NULL
		LEFT JOIN core.permissions p ON rp.permission_id = p.id AND p.deleted_at IS NULL
		WHERE u.email = $1 AND u.deleted_at IS NULL
		GROUP BY u.id
	`

	var user domain.User
	var rolesJSON, permissionsJSON []byte

	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
		&rolesJSON, &permissionsJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, nil // User not found
		}
		return nil, nil, nil, err
	}

	// Parse roles JSON
	var roles []string
	if len(rolesJSON) > 0 && string(rolesJSON) != "[]" {
		rolesStr := string(rolesJSON)
		rolesStr = rolesStr[1 : len(rolesStr)-1] // Remove [ ]
		if rolesStr != "" {
			parts := splitAndClean(rolesStr)
			roles = parts
		}
	}

	// Parse permissions JSON
	var permissions []string
	if len(permissionsJSON) > 0 && string(permissionsJSON) != "[]" {
		permStr := string(permissionsJSON)
		permStr = permStr[1 : len(permStr)-1] // Remove [ ]
		if permStr != "" {
			parts := splitAndClean(permStr)
			permissions = parts
		}
	}

	return &user, roles, permissions, nil
}

// FindByUsernameWithRoles finds a user by username and includes their roles and permissions
func (r *UserRepository) FindByUsernameWithRoles(username string) (*domain.User, []string, []string, error) {
	ctx := context.Background()
	query := `
		SELECT
			u.id, u.email, u.username, u.password_hash, u.full_name, u.phone,
			u.is_active, u.is_email_verified, u.last_login_at, u.created_at,
			u.created_by, u.updated_at, u.updated_by, u.deleted_at, u.deleted_by,
			COALESCE(
				json_agg(DISTINCT r.name) FILTER (WHERE r.name IS NOT NULL),
				'[]'
			) as roles,
			COALESCE(
				json_agg(DISTINCT (p.resource || ':' || p.action)) FILTER (WHERE p.resource IS NOT NULL),
				'[]'
			) as permissions
		FROM core.users u
		LEFT JOIN core.user_roles ur ON u.id = ur.user_id AND ur.deleted_at IS NULL
		LEFT JOIN core.roles r ON ur.role_id = r.id AND r.deleted_at IS NULL
		LEFT JOIN core.role_permissions rp ON r.id = rp.role_id AND rp.deleted_at IS NULL
		LEFT JOIN core.permissions p ON rp.permission_id = p.id AND p.deleted_at IS NULL
		WHERE u.username = $1 AND u.deleted_at IS NULL
		GROUP BY u.id
	`

	var user domain.User
	var rolesJSON, permissionsJSON []byte

	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.FullName, &user.Phone, &user.IsActive, &user.IsEmailVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.CreatedBy,
		&user.UpdatedAt, &user.UpdatedBy, &user.DeletedAt, &user.DeletedBy,
		&rolesJSON, &permissionsJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, nil // User not found
		}
		return nil, nil, nil, err
	}

	// Parse roles JSON
	var roles []string
	if len(rolesJSON) > 0 && string(rolesJSON) != "[]" {
		rolesStr := string(rolesJSON)
		rolesStr = rolesStr[1 : len(rolesStr)-1] // Remove [ ]
		if rolesStr != "" {
			parts := splitAndClean(rolesStr)
			roles = parts
		}
	}

	// Parse permissions JSON
	var permissions []string
	if len(permissionsJSON) > 0 && string(permissionsJSON) != "[]" {
		permStr := string(permissionsJSON)
		permStr = permStr[1 : len(permStr)-1] // Remove [ ]
		if permStr != "" {
			parts := splitAndClean(permStr)
			permissions = parts
		}
	}

	return &user, roles, permissions, nil
}

// AssignRole assigns a role to a user
func (r *UserRepository) AssignRole(userID, roleID, createdBy string) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.user_roles (user_id, role_id, created_by, updated_by)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (user_id, role_id) DO UPDATE
		SET deleted_at = NULL, deleted_by = NULL, updated_at = CURRENT_TIMESTAMP, updated_by = $3
	`
	_, err := r.db.Exec(ctx, query, userID, roleID, createdBy)
	return err
}

// RemoveRole removes a role from a user (soft delete)
func (r *UserRepository) RemoveRole(userID, roleID, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.user_roles
		SET deleted_at = CURRENT_TIMESTAMP, deleted_by = $3, updated_at = CURRENT_TIMESTAMP, updated_by = $3
		WHERE user_id = $1 AND role_id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.Exec(ctx, query, userID, roleID, deletedBy)
	return err
}

// SyncRoles replaces all user roles with new set (soft delete old, insert new)
func (r *UserRepository) SyncRoles(userID string, roleIDs []string, updatedBy string) error {
	ctx := context.Background()

	// Start transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Soft delete all existing roles
	deleteQuery := `
		UPDATE core.user_roles
		SET deleted_at = CURRENT_TIMESTAMP, deleted_by = $2, updated_at = CURRENT_TIMESTAMP, updated_by = $2
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	_, err = tx.Exec(ctx, deleteQuery, userID, updatedBy)
	if err != nil {
		return err
	}

	// Insert/restore new roles
	if len(roleIDs) > 0 {
		insertQuery := `
			INSERT INTO core.user_roles (user_id, role_id, created_by, updated_by)
			VALUES ($1, $2, $3, $3)
			ON CONFLICT (user_id, role_id) DO UPDATE
			SET deleted_at = NULL, deleted_by = NULL, updated_at = CURRENT_TIMESTAMP, updated_by = $3
		`
		for _, roleID := range roleIDs {
			_, err = tx.Exec(ctx, insertQuery, userID, roleID, updatedBy)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// AssignRoles assigns multiple roles to a user at once
func (r *UserRepository) AssignRoles(userID string, roleIDs []string, createdBy string) error {
	ctx := context.Background()

	if len(roleIDs) == 0 {
		return nil
	}

	// Use transaction for atomicity
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO core.user_roles (user_id, role_id, created_by, updated_by)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (user_id, role_id) DO UPDATE
		SET deleted_at = NULL, deleted_by = NULL, updated_at = CURRENT_TIMESTAMP, updated_by = $3
	`

	for _, roleID := range roleIDs {
		_, err = tx.Exec(ctx, query, userID, roleID, createdBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetUserRoleIDs gets all active role IDs for a user
func (r *UserRepository) GetUserRoleIDs(userID string) ([]string, error) {
	ctx := context.Background()
	query := `
		SELECT role_id
		FROM core.user_roles
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleIDs []string
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		roleIDs = append(roleIDs, roleID)
	}

	return roleIDs, nil
}

// Helper function to split and clean JSON array string
func splitAndClean(str string) []string {
	var result []string
	current := ""
	inQuote := false

	for _, char := range str {
		if char == '"' {
			inQuote = !inQuote
		} else if char == ',' && !inQuote {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if inQuote {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}