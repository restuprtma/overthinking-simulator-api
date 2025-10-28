package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"venturo-skeleton-go/internal/modules/core/permission_template/domain"
)

type PermissionTemplateRepository struct {
	db *pgxpool.Pool
}

func NewPermissionTemplateRepository(db *pgxpool.Pool) *PermissionTemplateRepository {
	return &PermissionTemplateRepository{db: db}
}

// FindByID retrieves a permission template by ID
func (r *PermissionTemplateRepository) FindByID(id string) (*domain.PermissionTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT pt.id, pt.name, pt.description, pt.is_system, pt.created_at, COALESCE(u1.full_name, pt.created_by::TEXT) as created_by,
		       pt.updated_at, COALESCE(u2.full_name, pt.updated_by::TEXT) as updated_by,
		       pt.deleted_at, COALESCE(u3.full_name, pt.deleted_by::TEXT) as deleted_by
		FROM core.permission_templates pt
		LEFT JOIN core.users u1 ON pt.created_by = u1.id
		LEFT JOIN core.users u2 ON pt.updated_by = u2.id
		LEFT JOIN core.users u3 ON pt.deleted_by = u3.id
		WHERE pt.id = $1 AND pt.deleted_at IS NULL
	`

	var template domain.PermissionTemplate
	err := r.db.QueryRow(ctx, query, id).Scan(
		&template.ID, &template.Name, &template.Description, &template.IsSystem,
		&template.CreatedAt, &template.CreatedBy, &template.UpdatedAt, &template.UpdatedBy,
		&template.DeletedAt, &template.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &template, nil
}

// FindByName retrieves a permission template by name
func (r *PermissionTemplateRepository) FindByName(name string) (*domain.PermissionTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT pt.id, pt.name, pt.description, pt.is_system, pt.created_at, COALESCE(u1.full_name, pt.created_by::TEXT) as created_by,
		       pt.updated_at, COALESCE(u2.full_name, pt.updated_by::TEXT) as updated_by,
		       pt.deleted_at, COALESCE(u3.full_name, pt.deleted_by::TEXT) as deleted_by
		FROM core.permission_templates pt
		LEFT JOIN core.users u1 ON pt.created_by = u1.id
		LEFT JOIN core.users u2 ON pt.updated_by = u2.id
		LEFT JOIN core.users u3 ON pt.deleted_by = u3.id
		WHERE pt.name = $1 AND pt.deleted_at IS NULL
	`

	var template domain.PermissionTemplate
	err := r.db.QueryRow(ctx, query, name).Scan(
		&template.ID, &template.Name, &template.Description, &template.IsSystem,
		&template.CreatedAt, &template.CreatedBy, &template.UpdatedAt, &template.UpdatedBy,
		&template.DeletedAt, &template.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &template, nil
}

// FindAll retrieves all permission templates with pagination and filters
func (r *PermissionTemplateRepository) FindAll(limit, offset int, search string, includeSystem *bool) ([]domain.PermissionTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT pt.id, pt.name, pt.description, pt.is_system, pt.created_at, COALESCE(u1.full_name, pt.created_by::TEXT) as created_by,
		       pt.updated_at, COALESCE(u2.full_name, pt.updated_by::TEXT) as updated_by,
		       pt.deleted_at, COALESCE(u3.full_name, pt.deleted_by::TEXT) as deleted_by
		FROM core.permission_templates pt
		LEFT JOIN core.users u1 ON pt.created_by = u1.id
		LEFT JOIN core.users u2 ON pt.updated_by = u2.id
		LEFT JOIN core.users u3 ON pt.deleted_by = u3.id
		WHERE pt.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(" AND (pt.name ILIKE $%d OR pt.description ILIKE $%d)", argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add system filter
	if includeSystem != nil && !*includeSystem {
		query += fmt.Sprintf(" AND pt.is_system = $%d", argPos)
		args = append(args, false)
		argPos++
	}

	query += " ORDER BY pt.created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.PermissionTemplate
	for rows.Next() {
		var template domain.PermissionTemplate
		err := rows.Scan(
			&template.ID, &template.Name, &template.Description, &template.IsSystem,
			&template.CreatedAt, &template.CreatedBy, &template.UpdatedAt, &template.UpdatedBy,
			&template.DeletedAt, &template.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

// Count returns the total count of permission templates
func (r *PermissionTemplateRepository) Count(search string, includeSystem *bool) (int64, error) {
	ctx := context.Background()
	query := `
		SELECT COUNT(*)
		FROM core.permission_templates pt
		WHERE pt.deleted_at IS NULL
	`

	args := []interface{}{}
	argPos := 1

	if search != "" {
		query += fmt.Sprintf(" AND (pt.name ILIKE $%d OR pt.description ILIKE $%d)", argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	if includeSystem != nil && !*includeSystem {
		query += fmt.Sprintf(" AND pt.is_system = $%d", argPos)
		args = append(args, false)
		argPos++
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create creates a new permission template
func (r *PermissionTemplateRepository) Create(template *domain.PermissionTemplate) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.permission_templates (id, name, description, is_system, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query,
		template.ID, template.Name, template.Description, template.IsSystem,
		template.CreatedAt, template.CreatedBy, template.UpdatedAt, template.UpdatedBy,
	)

	return err
}

// Update updates an existing permission template
func (r *PermissionTemplateRepository) Update(template *domain.PermissionTemplate) error {
	ctx := context.Background()
	query := `
		UPDATE core.permission_templates
		SET name = $1, description = $2, updated_at = $3, updated_by = $4
		WHERE id = $5 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		template.Name, template.Description, template.UpdatedAt, template.UpdatedBy,
		template.ID,
	)

	return err
}

// Delete soft deletes a permission template
func (r *PermissionTemplateRepository) Delete(templateID string, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.permission_templates
		SET deleted_at = NOW(), deleted_by = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, deletedBy, templateID)
	return err
}

// Restore restores a soft deleted permission template
func (r *PermissionTemplateRepository) Restore(templateID string) error {
	ctx := context.Background()
	query := `
		UPDATE core.permission_templates
		SET deleted_at = NULL, deleted_by = NULL
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, templateID)
	return err
}

// GetTemplateActions retrieves all actions for a template
func (r *PermissionTemplateRepository) GetTemplateActions(templateID string) ([]string, error) {
	ctx := context.Background()
	query := `
		SELECT pta.action
		FROM core.permission_template_actions pta
		WHERE pta.permission_template_id = $1
		ORDER BY pta.action
	`

	rows, err := r.db.Query(ctx, query, templateID)
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

// CreateTemplateAction creates a new action for a template
func (r *PermissionTemplateRepository) CreateTemplateAction(action *domain.PermissionTemplateAction) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.permission_template_actions (permission_template_id, action, created_at, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (permission_template_id, action) DO NOTHING
	`

	_, err := r.db.Exec(ctx, query,
		action.PermissionTemplateID, action.Action, action.CreatedAt, action.CreatedBy,
	)

	return err
}

// DeleteTemplateActions deletes all actions for a template
func (r *PermissionTemplateRepository) DeleteTemplateActions(templateID string) error {
	ctx := context.Background()
	query := `
		DELETE FROM core.permission_template_actions pta
		WHERE pta.permission_template_id = $1
	`

	_, err := r.db.Exec(ctx, query, templateID)
	return err
}
