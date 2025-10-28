package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lakukan-be/internal/modules/core/permission/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PermissionRepository struct {
	db *pgxpool.Pool
}

func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// FindByID finds a permission by ID
func (r *PermissionRepository) FindByID(id string) (*domain.Permission, error) {
	ctx := context.Background()
	query := `
		SELECT p.id, p.module_id, p.resource, p.action, p.description,
		       p.created_at, COALESCE(u1.full_name, p.created_by::TEXT) as created_by,
		       p.updated_at, COALESCE(u2.full_name, p.updated_by::TEXT) as updated_by,
		       p.deleted_at, COALESCE(u3.full_name, p.deleted_by::TEXT) as deleted_by,
		       m.id as module_id, m.code as module_code, m.name as module_name,
		       m.description as module_description, m.icon as module_icon,
		       m.color as module_color, m.sort_order as module_sort_order, m.is_active as module_is_active
		FROM core.permissions p
		LEFT JOIN core.modules m ON p.module_id = m.id AND m.deleted_at IS NULL
		LEFT JOIN core.users u1 ON p.created_by = u1.id
		LEFT JOIN core.users u2 ON p.updated_by = u2.id
		LEFT JOIN core.users u3 ON p.deleted_by = u3.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`

	var permission domain.Permission
	var module domain.Module
	err := r.db.QueryRow(ctx, query, id).Scan(
		&permission.ID, &permission.ModuleID, &permission.Resource, &permission.Action, &permission.Description,
		&permission.CreatedAt, &permission.CreatedBy, &permission.UpdatedAt, &permission.UpdatedBy,
		&permission.DeletedAt, &permission.DeletedBy,
		&module.ID, &module.Code, &module.Name, &module.Description, &module.Icon,
		&module.Color, &module.SortOrder, &module.IsActive,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Permission not found
		}
		return nil, err
	}

	permission.Module = &module
	return &permission, nil
}

// FindAll finds all permissions with pagination and filters
func (r *PermissionRepository) FindAll(limit, offset int, search, moduleID, resource, action string) ([]domain.Permission, error) {
	ctx := context.Background()
	query := `
		SELECT p.id, p.module_id, p.resource, p.action, p.description,
		       p.created_at, COALESCE(u1.full_name, p.created_by::TEXT) as created_by,
		       p.updated_at, COALESCE(u2.full_name, p.updated_by::TEXT) as updated_by,
		       p.deleted_at, COALESCE(u3.full_name, p.deleted_by::TEXT) as deleted_by,
		       m.id as module_id, m.code as module_code, m.name as module_name,
		       m.description as module_description, m.icon as module_icon,
		       m.color as module_color, m.sort_order as module_sort_order, m.is_active as module_is_active
		FROM core.permissions p
		LEFT JOIN core.modules m ON p.module_id = m.id AND m.deleted_at IS NULL
		LEFT JOIN core.users u1 ON p.created_by = u1.id
		LEFT JOIN core.users u2 ON p.updated_by = u2.id
		LEFT JOIN core.users u3 ON p.deleted_by = u3.id
		WHERE p.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter (search in resource, action, or description)
	if search != "" {
		query += fmt.Sprintf(` AND (p.resource ILIKE $%d OR p.action ILIKE $%d OR p.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add module filter
	if moduleID != "" {
		query += fmt.Sprintf(` AND p.module_id = $%d`, argPos)
		args = append(args, moduleID)
		argPos++
	}

	// Add resource filter
	if resource != "" {
		query += fmt.Sprintf(` AND p.resource = $%d`, argPos)
		args = append(args, resource)
		argPos++
	}

	// Add action filter
	if action != "" {
		query += fmt.Sprintf(` AND p.action = $%d`, argPos)
		args = append(args, action)
		argPos++
	}

	// Add ordering and pagination
	query += ` ORDER BY m.sort_order ASC, p.resource ASC, p.action ASC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []domain.Permission
	for rows.Next() {
		var permission domain.Permission
		var module domain.Module
		err := rows.Scan(
			&permission.ID, &permission.ModuleID, &permission.Resource, &permission.Action, &permission.Description,
			&permission.CreatedAt, &permission.CreatedBy, &permission.UpdatedAt, &permission.UpdatedBy,
			&permission.DeletedAt, &permission.DeletedBy,
			&module.ID, &module.Code, &module.Name, &module.Description, &module.Icon,
			&module.Color, &module.SortOrder, &module.IsActive,
		)
		if err != nil {
			return nil, err
		}
		permission.Module = &module
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// Count counts total permissions with filters
func (r *PermissionRepository) Count(search, moduleID, resource, action string) (int64, error) {
	ctx := context.Background()
	query := `
		SELECT COUNT(*)
		FROM core.permissions p
		WHERE p.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (p.resource ILIKE $%d OR p.action ILIKE $%d OR p.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add module filter
	if moduleID != "" {
		query += fmt.Sprintf(` AND p.module_id = $%d`, argPos)
		args = append(args, moduleID)
		argPos++
	}

	// Add resource filter
	if resource != "" {
		query += fmt.Sprintf(` AND p.resource = $%d`, argPos)
		args = append(args, resource)
		argPos++
	}

	// Add action filter
	if action != "" {
		query += fmt.Sprintf(` AND p.action = $%d`, argPos)
		args = append(args, action)
		argPos++
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CheckDuplicateResourceAction checks if resource-action combination already exists
func (r *PermissionRepository) CheckDuplicateResourceAction(moduleID, resource, action string, excludeID *string) (bool, error) {
	ctx := context.Background()
	query := `
		SELECT EXISTS(
			SELECT 1 FROM core.permissions p
			WHERE p.module_id = $1 AND p.resource = $2 AND p.action = $3 AND p.deleted_at IS NULL
	`

	args := []interface{}{moduleID, resource, action}

	if excludeID != nil {
		query += " AND p.id != $4"
		args = append(args, *excludeID)
	}

	query += ")"

	var exists bool
	err := r.db.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Create creates a new permission
func (r *PermissionRepository) Create(permission *domain.Permission) error {
	ctx := context.Background()

	// Generate UUID if not provided
	if permission.ID == "" {
		permission.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	permission.CreatedAt = now
	permission.UpdatedAt = now

	query := `
		INSERT INTO core.permissions (id, module_id, resource, action, description, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(ctx, query,
		permission.ID, permission.ModuleID, permission.Resource, permission.Action, permission.Description,
		permission.CreatedAt, permission.CreatedBy, permission.UpdatedAt, permission.UpdatedBy,
	)

	return err
}

// Update updates a permission
func (r *PermissionRepository) Update(permission *domain.Permission) error {
	ctx := context.Background()

	// Set updated timestamp
	permission.UpdatedAt = time.Now()

	query := `
		UPDATE core.permissions
		SET module_id = $1, resource = $2, action = $3, description = $4, updated_at = $5, updated_by = $6
		WHERE id = $7 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query,
		permission.ModuleID, permission.Resource, permission.Action, permission.Description,
		permission.UpdatedAt, permission.UpdatedBy, permission.ID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("permission not found or already deleted")
	}

	return nil
}

// Delete soft deletes a permission
func (r *PermissionRepository) Delete(id, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.permissions
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, time.Now(), deletedBy, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("permission not found or already deleted")
	}

	return nil
}

// GetDistinctActions retrieves all unique action names
func (r *PermissionRepository) GetDistinctActions() ([]string, error) {
	ctx := context.Background()
	query := `
		SELECT DISTINCT p.action
		FROM core.permissions p
		WHERE p.deleted_at IS NULL
		ORDER BY p.action
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	return actions, nil
}
