package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"venturo-skeleton-go/internal/modules/core/role/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

// FindByID finds a role by ID
func (r *RoleRepository) FindByID(id string) (*domain.Role, error) {
	ctx := context.Background()
	query := `
		SELECT r.id, r.name, r.description, r.is_system, r.created_at, COALESCE(u1.full_name, r.created_by::TEXT) as created_by,
		       r.updated_at, COALESCE(u2.full_name, r.updated_by::TEXT) as updated_by,
		       r.deleted_at, COALESCE(u3.full_name, r.deleted_by::TEXT) as deleted_by
		FROM core.roles r
		LEFT JOIN core.users u1 ON r.created_by = u1.id
		LEFT JOIN core.users u2 ON r.updated_by = u2.id
		LEFT JOIN core.users u3 ON r.deleted_by = u3.id
		WHERE r.id = $1 AND r.deleted_at IS NULL
	`

	var role domain.Role
	err := r.db.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystem,
		&role.CreatedAt, &role.CreatedBy, &role.UpdatedAt, &role.UpdatedBy,
		&role.DeletedAt, &role.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Role not found
		}
		return nil, err
	}

	return &role, nil
}

// FindByName finds a role by name
func (r *RoleRepository) FindByName(name string) (*domain.Role, error) {
	ctx := context.Background()
	query := `
		SELECT r.id, r.name, r.description, r.is_system, r.created_at, COALESCE(u1.full_name, r.created_by::TEXT) as created_by,
		       r.updated_at, COALESCE(u2.full_name, r.updated_by::TEXT) as updated_by,
		       r.deleted_at, COALESCE(u3.full_name, r.deleted_by::TEXT) as deleted_by
		FROM core.roles r
		LEFT JOIN core.users u1 ON r.created_by = u1.id
		LEFT JOIN core.users u2 ON r.updated_by = u2.id
		LEFT JOIN core.users u3 ON r.deleted_by = u3.id
		WHERE r.name = $1 AND r.deleted_at IS NULL
	`

	var role domain.Role
	err := r.db.QueryRow(ctx, query, name).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystem,
		&role.CreatedAt, &role.CreatedBy, &role.UpdatedAt, &role.UpdatedBy,
		&role.DeletedAt, &role.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Role not found
		}
		return nil, err
	}

	return &role, nil
}

// FindAll finds all roles with pagination and filters
func (r *RoleRepository) FindAll(limit, offset int, search string, includeSystem *bool) ([]domain.Role, error) {
	ctx := context.Background()
	query := `
		SELECT r.id, r.name, r.description, r.is_system, r.created_at, COALESCE(u1.full_name, r.created_by::TEXT) as created_by,
		       r.updated_at, COALESCE(u2.full_name, r.updated_by::TEXT) as updated_by,
		       r.deleted_at, COALESCE(u3.full_name, r.deleted_by::TEXT) as deleted_by
		FROM core.roles r
		LEFT JOIN core.users u1 ON r.created_by = u1.id
		LEFT JOIN core.users u2 ON r.updated_by = u2.id
		LEFT JOIN core.users u3 ON r.deleted_by = u3.id
		WHERE r.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (r.name ILIKE $%d OR r.description ILIKE $%d)`, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add includeSystem filter
	if includeSystem != nil && !*includeSystem {
		query += fmt.Sprintf(` AND r.is_system = false`)
	}

	query += fmt.Sprintf(` ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.IsSystem,
			&role.CreatedAt, &role.CreatedBy, &role.UpdatedAt, &role.UpdatedBy,
			&role.DeletedAt, &role.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// Count counts total roles with filters
func (r *RoleRepository) Count(search string, includeSystem *bool) (int64, error) {
	ctx := context.Background()
	query := `SELECT COUNT(*) FROM core.roles r WHERE r.deleted_at IS NULL`

	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (r.name ILIKE $%d OR r.description ILIKE $%d)`, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add includeSystem filter
	if includeSystem != nil && !*includeSystem {
		query += ` AND r.is_system = false`
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// Create creates a new role
func (r *RoleRepository) Create(role *domain.Role) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.roles (
			id, name, description, is_system, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query,
		role.ID, role.Name, role.Description, role.IsSystem,
		role.CreatedAt, role.CreatedBy, role.UpdatedAt, role.UpdatedBy,
	)

	return err
}

// Update updates a role
func (r *RoleRepository) Update(role *domain.Role) error {
	ctx := context.Background()
	query := `
		UPDATE core.roles
		SET name = $1, description = $2, updated_at = $3, updated_by = $4
		WHERE id = $5 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		role.Name, role.Description, role.UpdatedAt, role.UpdatedBy, role.ID,
	)

	return err
}

// Delete soft deletes a role
func (r *RoleRepository) Delete(roleID string, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.roles
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), deletedBy, roleID)
	return err
}

// Restore restores a soft-deleted role
func (r *RoleRepository) Restore(roleID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.roles
		SET deleted_at = NULL, deleted_by = NULL, updated_at = $1
		WHERE id = $2 AND deleted_at IS NOT NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), roleID)
	return err
}

// Permission represents a permission entity
type Permission struct {
	ID          string
	Resource    string
	Action      string
	Description string
}

// GetPermissionsByRoleID gets all permissions for a specific role
func (r *RoleRepository) GetPermissionsByRoleID(roleID string) ([]Permission, error) {
	ctx := context.Background()
	query := `
		SELECT p.id, p.resource, p.action, p.description
		FROM core.permissions p
		INNER JOIN core.role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1 AND rp.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY p.resource, p.action
	`

	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var perm Permission
		err := rows.Scan(&perm.ID, &perm.Resource, &perm.Action, &perm.Description)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// ClearPermissions removes all permissions from a role
func (r *RoleRepository) ClearPermissions(roleID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.role_permissions
		SET deleted_at = $1
		WHERE role_id = $2 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), roleID)
	return err
}

// AssignPermission assigns a single permission to a role
func (r *RoleRepository) AssignPermission(roleID string, permissionID string) error {
	ctx := context.Background()

	// First check if relation already exists (including soft-deleted ones)
	checkQuery := `
		SELECT deleted_at FROM core.role_permissions
		WHERE role_id = $1 AND permission_id = $2
	`

	var deletedAt *time.Time
	err := r.db.QueryRow(ctx, checkQuery, roleID, permissionID).Scan(&deletedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Relation doesn't exist, create new one
			insertQuery := `
				INSERT INTO core.role_permissions (role_id, permission_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4)
			`
			_, err = r.db.Exec(ctx, insertQuery, roleID, permissionID, time.Now(), time.Now())
			return err
		}
		return err
	}

	// Relation exists, check if it's soft-deleted
	if deletedAt != nil {
		// Restore the soft-deleted relation
		updateQuery := `
			UPDATE core.role_permissions
			SET deleted_at = NULL, updated_at = $1
			WHERE role_id = $2 AND permission_id = $3
		`
		_, err = r.db.Exec(ctx, updateQuery, time.Now(), roleID, permissionID)
		return err
	}

	// Relation already exists and is active, no action needed
	return nil
}

// AssignPermissions assigns multiple permissions to a role (replacing all existing)
func (r *RoleRepository) AssignPermissions(roleID string, permissionIDs []string) error {
	ctx := context.Background()

	// Start transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Clear existing permissions (soft delete)
	clearQuery := `
		UPDATE core.role_permissions
		SET deleted_at = $1
		WHERE role_id = $2 AND deleted_at IS NULL
	`
	_, err = tx.Exec(ctx, clearQuery, time.Now(), roleID)
	if err != nil {
		return err
	}

	// Assign new permissions
	for _, permissionID := range permissionIDs {
		// Check if relation exists
		checkQuery := `
			SELECT deleted_at FROM core.role_permissions
			WHERE role_id = $1 AND permission_id = $2
		`

		var deletedAt *time.Time
		err = tx.QueryRow(ctx, checkQuery, roleID, permissionID).Scan(&deletedAt)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Insert new relation
				insertQuery := `
					INSERT INTO core.role_permissions (role_id, permission_id, created_at, updated_at)
					VALUES ($1, $2, $3, $4)
				`
				_, err = tx.Exec(ctx, insertQuery, roleID, permissionID, time.Now(), time.Now())
				if err != nil {
					return err
				}
			} else {
				return err
			}
		} else if deletedAt != nil {
			// Restore soft-deleted relation
			updateQuery := `
				UPDATE core.role_permissions
				SET deleted_at = NULL, updated_at = $1
				WHERE role_id = $2 AND permission_id = $3
			`
			_, err = tx.Exec(ctx, updateQuery, time.Now(), roleID, permissionID)
			if err != nil {
				return err
			}
		}
		// If relation exists and is active, skip
	}

	return tx.Commit(ctx)
}

// RemovePermission removes a single permission from a role (soft delete)
func (r *RoleRepository) RemovePermission(roleID string, permissionID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.role_permissions
		SET deleted_at = $1
		WHERE role_id = $2 AND permission_id = $3 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), roleID, permissionID)
	return err
}
