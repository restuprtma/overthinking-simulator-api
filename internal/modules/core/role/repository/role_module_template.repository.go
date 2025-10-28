package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"lakukan-be/internal/modules/core/role/domain"
)

type RoleModuleTemplateRepository struct {
	db *pgxpool.Pool
}

func NewRoleModuleTemplateRepository(db *pgxpool.Pool) *RoleModuleTemplateRepository {
	return &RoleModuleTemplateRepository{db: db}
}

// FindByRoleID retrieves all module templates for a role
func (r *RoleModuleTemplateRepository) FindByRoleID(roleID string) ([]domain.RoleModuleTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT id, role_id, module_id, permission_template_id, created_at, created_by,
		       updated_at, updated_by, deleted_at, deleted_by
		FROM core.role_module_templates
		WHERE role_id = $1 AND deleted_at IS NULL
		ORDER BY module_id
	`

	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.RoleModuleTemplate
	for rows.Next() {
		var template domain.RoleModuleTemplate
		err := rows.Scan(
			&template.ID, &template.RoleID, &template.ModuleID, &template.PermissionTemplateID,
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

// Create creates a new role module template mapping
func (r *RoleModuleTemplateRepository) Create(template *domain.RoleModuleTemplate) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.role_module_templates
		(id, role_id, module_id, permission_template_id, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query,
		template.ID, template.RoleID, template.ModuleID, template.PermissionTemplateID,
		template.CreatedAt, template.CreatedBy, template.UpdatedAt, template.UpdatedBy,
	)

	return err
}

// DeleteByRoleID deletes all module template mappings for a role
func (r *RoleModuleTemplateRepository) DeleteByRoleID(roleID string) error {
	ctx := context.Background()
	query := `
		DELETE FROM core.role_module_templates
		WHERE role_id = $1
	`

	_, err := r.db.Exec(ctx, query, roleID)
	return err
}

// BulkCreate creates multiple role module template mappings efficiently
// moduleTemplates is a map of module_id -> permission_template_id
func (r *RoleModuleTemplateRepository) BulkCreate(roleID string, moduleTemplates map[string]string, createdBy string) error {
	if len(moduleTemplates) == 0 {
		return nil
	}

	ctx := context.Background()
	now := time.Now()

	// Use batch insert for efficiency
	batch := &[][]interface{}{}
	for moduleID, permissionTemplateID := range moduleTemplates {
		*batch = append(*batch, []interface{}{
			uuid.New().String(),
			roleID,
			moduleID,
			permissionTemplateID,
			now,
			createdBy,
			now,
			createdBy,
		})
	}

	// Build the batch insert query
	query := `
		INSERT INTO core.role_module_templates
		(id, role_id, module_id, permission_template_id, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for _, values := range *batch {
		_, err := r.db.Exec(ctx, query, values...)
		if err != nil {
			return err
		}
	}

	return nil
}

// FindByRoleIDWithTemplateNames retrieves role module templates with template and module names
func (r *RoleModuleTemplateRepository) FindByRoleIDWithTemplateNames(roleID string) ([]map[string]interface{}, error) {
	ctx := context.Background()
	query := `
		SELECT
			rmt.module_id,
			m.code as module_code,
			m.name as module_name,
			rmt.permission_template_id,
			pt.name as permission_template_name
		FROM core.role_module_templates rmt
		JOIN core.modules m ON rmt.module_id = m.id
		JOIN core.permission_templates pt ON rmt.permission_template_id = pt.id
		WHERE rmt.role_id = $1
		  AND rmt.deleted_at IS NULL
		  AND m.deleted_at IS NULL
		  AND pt.deleted_at IS NULL
		ORDER BY m.sort_order, m.name
	`

	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var moduleID, moduleCode, moduleName, permissionTemplateID, permissionTemplateName string
		err := rows.Scan(&moduleID, &moduleCode, &moduleName, &permissionTemplateID, &permissionTemplateName)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"module_id":                  moduleID,
			"module_code":                moduleCode,
			"module_name":                moduleName,
			"permission_template_id":     permissionTemplateID,
			"permission_template_name":   permissionTemplateName,
		})
	}

	return results, nil
}
