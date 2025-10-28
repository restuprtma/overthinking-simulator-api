package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"venturo-skeleton-go/internal/modules/core/permission/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModuleRepository struct {
	db *pgxpool.Pool
}

func NewModuleRepository(db *pgxpool.Pool) *ModuleRepository {
	return &ModuleRepository{db: db}
}

// FindByID finds a module by ID
func (r *ModuleRepository) FindByID(id string) (*domain.Module, error) {
	ctx := context.Background()
	query := `
		SELECT m.id, m.code, m.name, m.description, m.icon, m.color, m.sort_order, m.is_active,
		       m.parent_id, m.depth, m.heading, m."to", m.url, m.permission, m.tooltip, m.is_menu_visible,
		       COALESCE(perm_count.count, 0) as permissions_count,
		       m.created_at, COALESCE(u1.full_name, m.created_by::TEXT) as created_by,
		       m.updated_at, COALESCE(u2.full_name, m.updated_by::TEXT) as updated_by,
		       m.deleted_at, COALESCE(u3.full_name, m.deleted_by::TEXT) as deleted_by
		FROM core.modules m
		LEFT JOIN core.users u1 ON m.created_by = u1.id
		LEFT JOIN core.users u2 ON m.updated_by = u2.id
		LEFT JOIN core.users u3 ON m.deleted_by = u3.id
		LEFT JOIN (
			SELECT module_id, COUNT(*) as count
			FROM core.permissions
			WHERE deleted_at IS NULL
			GROUP BY module_id
		) perm_count ON perm_count.module_id = m.id
		WHERE m.id = $1 AND m.deleted_at IS NULL
	`

	var module domain.Module
	err := r.db.QueryRow(ctx, query, id).Scan(
		&module.ID, &module.Code, &module.Name, &module.Description, &module.Icon,
		&module.Color, &module.SortOrder, &module.IsActive,
		&module.ParentID, &module.Depth, &module.Heading, &module.To, &module.URL,
		&module.Permission, &module.Tooltip, &module.IsMenuVisible,
		&module.PermissionsCount,
		&module.CreatedAt, &module.CreatedBy, &module.UpdatedAt, &module.UpdatedBy,
		&module.DeletedAt, &module.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Module not found
		}
		return nil, err
	}

	return &module, nil
}

// FindAll finds all modules with filters (no pagination)
// Returns modules in hierarchical order: parent followed immediately by its children
func (r *ModuleRepository) FindAll(search string, isActive *bool) ([]domain.Module, error) {
	ctx := context.Background()

	// Build WHERE conditions for the CTE
	whereConditions := "m.deleted_at IS NULL"
	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		whereConditions += fmt.Sprintf(` AND (m.code ILIKE $%d OR m.name ILIKE $%d OR m.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add is_active filter
	if isActive != nil {
		whereConditions += fmt.Sprintf(` AND m.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	// Recursive CTE for hierarchical ordering
	// This ensures parent appears immediately before its children
	query := fmt.Sprintf(`
		WITH RECURSIVE module_tree AS (
			-- Base case: root modules (parent_id IS NULL)
			SELECT
				m.id, m.code, m.name, m.description, m.icon, m.color, m.sort_order, m.is_active,
				m.parent_id, m.depth, m.heading, m."to", m.url, m.permission, m.tooltip, m.is_menu_visible,
				m.created_at, m.created_by, m.updated_at, m.updated_by, m.deleted_at, m.deleted_by,
				m.id::TEXT as path,  -- Path for ordering
				m.sort_order as root_sort_order
			FROM core.modules m
			WHERE %s AND m.parent_id IS NULL

			UNION ALL

			-- Recursive case: children of current level
			SELECT
				m.id, m.code, m.name, m.description, m.icon, m.color, m.sort_order, m.is_active,
				m.parent_id, m.depth, m.heading, m."to", m.url, m.permission, m.tooltip, m.is_menu_visible,
				m.created_at, m.created_by, m.updated_at, m.updated_by, m.deleted_at, m.deleted_by,
				mt.path || '/' || m.id::TEXT as path,
				mt.root_sort_order
			FROM core.modules m
			INNER JOIN module_tree mt ON m.parent_id = mt.id
			WHERE %s
		)
		SELECT
			mt.id, mt.code, mt.name, mt.description, mt.icon, mt.color, mt.sort_order, mt.is_active,
			mt.parent_id, mt.depth, mt.heading, mt."to", mt.url, mt.permission, mt.tooltip, mt.is_menu_visible,
			COALESCE(perm_count.count, 0) as permissions_count,
			mt.created_at, COALESCE(u1.full_name, mt.created_by::TEXT) as created_by,
			mt.updated_at, COALESCE(u2.full_name, mt.updated_by::TEXT) as updated_by,
			mt.deleted_at, COALESCE(u3.full_name, mt.deleted_by::TEXT) as deleted_by
		FROM module_tree mt
		LEFT JOIN core.users u1 ON mt.created_by = u1.id
		LEFT JOIN core.users u2 ON mt.updated_by = u2.id
		LEFT JOIN core.users u3 ON mt.deleted_by = u3.id
		LEFT JOIN (
			SELECT module_id, COUNT(*) as count
			FROM core.permissions
			WHERE deleted_at IS NULL
			GROUP BY module_id
		) perm_count ON perm_count.module_id = mt.id
		ORDER BY mt.root_sort_order, mt.path, mt.sort_order
	`, whereConditions, whereConditions)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []domain.Module
	for rows.Next() {
		var module domain.Module
		err := rows.Scan(
			&module.ID, &module.Code, &module.Name, &module.Description, &module.Icon,
			&module.Color, &module.SortOrder, &module.IsActive,
			&module.ParentID, &module.Depth, &module.Heading, &module.To, &module.URL,
			&module.Permission, &module.Tooltip, &module.IsMenuVisible,
			&module.PermissionsCount,
			&module.CreatedAt, &module.CreatedBy, &module.UpdatedAt, &module.UpdatedBy,
			&module.DeletedAt, &module.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}

	return modules, nil
}

// Count counts total modules with filters
func (r *ModuleRepository) Count(search string, isActive *bool) (int64, error) {
	ctx := context.Background()
	query := `
		SELECT COUNT(*)
		FROM core.modules m
		WHERE m.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (m.code ILIKE $%d OR m.name ILIKE $%d OR m.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add is_active filter
	if isActive != nil {
		query += fmt.Sprintf(` AND m.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CheckDuplicateCode checks if code already exists
func (r *ModuleRepository) CheckDuplicateCode(code string, excludeID *string) (bool, error) {
	ctx := context.Background()
	query := `
		SELECT EXISTS(
			SELECT 1 FROM core.modules m
			WHERE m.code = $1 AND m.deleted_at IS NULL
	`

	args := []interface{}{code}

	if excludeID != nil {
		query += " AND m.id != $2"
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

// Create creates a new module
func (r *ModuleRepository) Create(module *domain.Module) error {
	ctx := context.Background()

	// Generate UUID if not provided
	if module.ID == "" {
		module.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	module.CreatedAt = now
	module.UpdatedAt = now

	query := `
		INSERT INTO core.modules (
			id, code, name, description, icon, color, sort_order, is_active,
			parent_id, depth, heading, "to", url, permission, tooltip, is_menu_visible,
			created_at, created_by, updated_at, updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`

	_, err := r.db.Exec(ctx, query,
		module.ID, module.Code, module.Name, module.Description, module.Icon,
		module.Color, module.SortOrder, module.IsActive,
		module.ParentID, module.Depth, module.Heading, module.To, module.URL,
		module.Permission, module.Tooltip, module.IsMenuVisible,
		module.CreatedAt, module.CreatedBy, module.UpdatedAt, module.UpdatedBy,
	)

	return err
}

// Update updates a module
func (r *ModuleRepository) Update(module *domain.Module) error {
	ctx := context.Background()

	// Set updated timestamp
	module.UpdatedAt = time.Now()

	query := `
		UPDATE core.modules
		SET code = $1, name = $2, description = $3, icon = $4, color = $5,
		    sort_order = $6, is_active = $7,
		    parent_id = $8, depth = $9, heading = $10, "to" = $11, url = $12,
		    permission = $13, tooltip = $14, is_menu_visible = $15,
		    updated_at = $16, updated_by = $17
		WHERE id = $18 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query,
		module.Code, module.Name, module.Description, module.Icon, module.Color,
		module.SortOrder, module.IsActive,
		module.ParentID, module.Depth, module.Heading, module.To, module.URL,
		module.Permission, module.Tooltip, module.IsMenuVisible,
		module.UpdatedAt, module.UpdatedBy, module.ID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("module not found or already deleted")
	}

	return nil
}

// Delete soft deletes a module
func (r *ModuleRepository) Delete(id, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.modules
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, time.Now(), deletedBy, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("module not found or already deleted")
	}

	return nil
}

// Restore restores a soft-deleted module
func (r *ModuleRepository) Restore(id string) error {
	ctx := context.Background()
	query := `
		UPDATE core.modules
		SET deleted_at = NULL, deleted_by = NULL, updated_at = $1
		WHERE id = $2 AND deleted_at IS NOT NULL
	`

	result, err := r.db.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("module not found or not deleted")
	}

	return nil
}
