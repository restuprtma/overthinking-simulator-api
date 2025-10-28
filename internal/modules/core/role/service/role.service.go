package service

import (
	"context"
	"errors"
	"math"
	"time"

	"venturo-skeleton-go/internal/modules/core/role/domain"
	"venturo-skeleton-go/internal/modules/core/role/dto"
	"venturo-skeleton-go/internal/modules/core/role/repository"
	"venturo-skeleton-go/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRoleNotFound          = errors.New("role not found")
	ErrRoleNameAlreadyExists = errors.New("role name already exists")
	ErrSystemRoleCannotModify = errors.New("system role cannot be modified or deleted")
)

type RoleService struct {
	roleRepo               *repository.RoleRepository
	roleModuleTemplateRepo *repository.RoleModuleTemplateRepository
	db                     *pgxpool.Pool
}

func NewRoleService(roleRepo *repository.RoleRepository, roleModuleTemplateRepo *repository.RoleModuleTemplateRepository, db *pgxpool.Pool) *RoleService {
	return &RoleService{
		roleRepo:               roleRepo,
		roleModuleTemplateRepo: roleModuleTemplateRepo,
		db:                     db,
	}
}

// GetAll gets all roles with pagination
func (s *RoleService) GetAll(params *dto.RoleQueryParams) (*dto.RoleListResponse, error) {
	logger.Info("Fetching roles list",
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
		logger.String("search", params.Search),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get roles
	roles, err := s.roleRepo.FindAll(params.PageSize, offset, params.Search, params.IncludeSystem)
	if err != nil {
		logger.Error("Failed to fetch roles", logger.Err(err))
		return nil, err
	}

	// Get total count
	total, err := s.roleRepo.Count(params.Search, params.IncludeSystem)
	if err != nil {
		logger.Error("Failed to count roles", logger.Err(err))
		return nil, err
	}

	// Convert to response
	roleResponses := make([]dto.RoleResponse, len(roles))
	for i, role := range roles {
		roleResponses[i] = s.toRoleResponse(&role)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.RoleListResponse{
		Roles:      roleResponses,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID gets a role by ID
func (s *RoleService) GetByID(id string) (*dto.RoleResponse, error) {
	logger.Info("Fetching role by ID", logger.String("role_id", id))

	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return nil, err
	}

	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", id))
		return nil, ErrRoleNotFound
	}

	response := s.toRoleResponse(role)
	return &response, nil
}

// Create creates a new role
func (s *RoleService) Create(req *dto.CreateRoleRequest, createdBy string) (*dto.RoleResponse, error) {
	logger.Info("Creating new role", logger.String("name", req.Name))

	// Check if name already exists
	existingRole, err := s.roleRepo.FindByName(req.Name)
	if err != nil {
		logger.Error("Failed to check existing role name", logger.Err(err))
		return nil, err
	}
	if existingRole != nil {
		logger.Warn("Role name already exists", logger.String("name", req.Name))
		return nil, ErrRoleNameAlreadyExists
	}

	// Create role
	now := time.Now()
	role := &domain.Role{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false, // User-created roles are never system roles
		CreatedAt:   now,
		CreatedBy:   &createdBy,
		UpdatedAt:   now,
		UpdatedBy:   &createdBy,
	}

	err = s.roleRepo.Create(role)
	if err != nil {
		logger.Error("Failed to create role", logger.Err(err))
		return nil, err
	}

	// Handle module templates if provided
	if len(req.ModuleTemplates) > 0 {
		// Expand templates to permission IDs
		permissionIDs, err := s.expandTemplateToPermissions(req.ModuleTemplates)
		if err != nil {
			logger.Error("Failed to expand templates to permissions", logger.Err(err))
			return nil, err
		}

		// Assign permissions if any
		if len(permissionIDs) > 0 {
			err = s.roleRepo.AssignPermissions(role.ID, permissionIDs)
			if err != nil {
				logger.Error("Failed to assign permissions", logger.Err(err))
				return nil, err
			}
		}

		// Save module template mappings
		err = s.roleModuleTemplateRepo.BulkCreate(role.ID, req.ModuleTemplates, createdBy)
		if err != nil {
			logger.Error("Failed to save module templates", logger.Err(err))
			return nil, err
		}
	} else if len(req.Permissions) > 0 {
		// Backward compatibility: use direct permissions if provided
		err = s.roleRepo.AssignPermissions(role.ID, req.Permissions)
		if err != nil {
			logger.Error("Failed to assign permissions", logger.Err(err))
			return nil, err
		}
	}

	logger.Info("Role created successfully", logger.String("role_id", role.ID))

	response := s.toRoleResponse(role)
	return &response, nil
}

// Update updates a role
func (s *RoleService) Update(id string, req *dto.UpdateRoleRequest, updatedBy string) (*dto.RoleResponse, error) {
	logger.Info("Updating role", logger.String("role_id", id))

	// Find role
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return nil, err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", id))
		return nil, ErrRoleNotFound
	}

	// Check if it's a system role
	if role.IsSystem {
		logger.Warn("Cannot modify system role", logger.String("role_id", id))
		return nil, ErrSystemRoleCannotModify
	}

	// Update fields if provided
	if req.Name != nil && *req.Name != role.Name {
		// Check if new name already exists
		existingRole, err := s.roleRepo.FindByName(*req.Name)
		if err != nil {
			logger.Error("Failed to check existing role name", logger.Err(err))
			return nil, err
		}
		if existingRole != nil && existingRole.ID != id {
			logger.Warn("Role name already exists", logger.String("name", *req.Name))
			return nil, ErrRoleNameAlreadyExists
		}
		role.Name = *req.Name
	}

	if req.Description != nil {
		role.Description = req.Description
	}

	role.UpdatedAt = time.Now()
	role.UpdatedBy = &updatedBy

	// Update role
	err = s.roleRepo.Update(role)
	if err != nil {
		logger.Error("Failed to update role", logger.Err(err))
		return nil, err
	}

	// Handle module templates if provided
	if len(req.ModuleTemplates) > 0 {
		// Delete old module templates
		err = s.roleModuleTemplateRepo.DeleteByRoleID(id)
		if err != nil {
			logger.Error("Failed to delete old module templates", logger.Err(err))
			return nil, err
		}

		// Expand templates to permission IDs
		permissionIDs, err := s.expandTemplateToPermissions(req.ModuleTemplates)
		if err != nil {
			logger.Error("Failed to expand templates to permissions", logger.Err(err))
			return nil, err
		}

		// Replace permissions
		// First, remove all existing permissions
		ctx := context.Background()
		_, err = s.db.Exec(ctx, "DELETE FROM core.role_permissions WHERE role_id = $1", id)
		if err != nil {
			logger.Error("Failed to delete old permissions", logger.Err(err))
			return nil, err
		}

		// Assign new permissions
		if len(permissionIDs) > 0 {
			err = s.roleRepo.AssignPermissions(id, permissionIDs)
			if err != nil {
				logger.Error("Failed to assign new permissions", logger.Err(err))
				return nil, err
			}
		}

		// Save new module template mappings
		err = s.roleModuleTemplateRepo.BulkCreate(id, req.ModuleTemplates, updatedBy)
		if err != nil {
			logger.Error("Failed to save new module templates", logger.Err(err))
			return nil, err
		}
	} else if len(req.Permissions) > 0 {
		// Backward compatibility: use direct permissions if provided
		// Delete old module templates
		err = s.roleModuleTemplateRepo.DeleteByRoleID(id)
		if err != nil {
			logger.Error("Failed to delete old module templates", logger.Err(err))
			return nil, err
		}

		// Replace permissions
		ctx := context.Background()
		_, err = s.db.Exec(ctx, "DELETE FROM core.role_permissions WHERE role_id = $1", id)
		if err != nil {
			logger.Error("Failed to delete old permissions", logger.Err(err))
			return nil, err
		}

		err = s.roleRepo.AssignPermissions(id, req.Permissions)
		if err != nil {
			logger.Error("Failed to assign permissions", logger.Err(err))
			return nil, err
		}
	}

	logger.Info("Role updated successfully", logger.String("role_id", id))

	response := s.toRoleResponse(role)
	return &response, nil
}

// Delete deletes a role (soft delete)
func (s *RoleService) Delete(id string, deletedBy string) error {
	logger.Info("Deleting role", logger.String("role_id", id))

	// Check if role exists
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", id))
		return ErrRoleNotFound
	}

	// Check if it's a system role
	if role.IsSystem {
		logger.Warn("Cannot delete system role", logger.String("role_id", id))
		return ErrSystemRoleCannotModify
	}

	// Soft delete
	err = s.roleRepo.Delete(id, deletedBy)
	if err != nil {
		logger.Error("Failed to delete role", logger.Err(err))
		return err
	}

	logger.Info("Role deleted successfully", logger.String("role_id", id))
	return nil
}

// Restore restores a soft-deleted role
func (s *RoleService) Restore(id string) (*dto.RoleResponse, error) {
	logger.Info("Restoring role", logger.String("role_id", id))

	// Restore role
	err := s.roleRepo.Restore(id)
	if err != nil {
		logger.Error("Failed to restore role", logger.Err(err))
		return nil, err
	}

	// Fetch restored role
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch restored role", logger.Err(err))
		return nil, err
	}
	if role == nil {
		logger.Warn("Role not found after restore", logger.String("role_id", id))
		return nil, ErrRoleNotFound
	}

	logger.Info("Role restored successfully", logger.String("role_id", id))

	response := s.toRoleResponse(role)
	return &response, nil
}

// GetRoleWithPermissions gets role with its permissions
func (s *RoleService) GetRoleWithPermissions(id string) (*dto.RoleWithPermissionsResponse, error) {
	logger.Info("Fetching role with permissions", logger.String("role_id", id))

	// Get role
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return nil, err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", id))
		return nil, ErrRoleNotFound
	}

	// Get permissions
	permissions, err := s.roleRepo.GetPermissionsByRoleID(id)
	if err != nil {
		logger.Error("Failed to fetch role permissions", logger.Err(err))
		return nil, err
	}

	// Convert to response
	permResponses := make([]dto.PermissionResponse, len(permissions))
	for i, perm := range permissions {
		permResponses[i] = dto.PermissionResponse{
			ID:          perm.ID,
			Resource:    perm.Resource,
			Action:      perm.Action,
			Description: perm.Description,
		}
	}

	// Get module templates
	moduleTemplateData, err := s.roleModuleTemplateRepo.FindByRoleIDWithTemplateNames(id)
	if err != nil {
		logger.Error("Failed to fetch module templates", logger.Err(err))
		return nil, err
	}

	// Convert to response
	var moduleTemplates []dto.RoleModuleTemplateResponse
	for _, data := range moduleTemplateData {
		moduleTemplates = append(moduleTemplates, dto.RoleModuleTemplateResponse{
			ModuleID:               data["module_id"].(string),
			ModuleCode:             data["module_code"].(string),
			ModuleName:             data["module_name"].(string),
			PermissionTemplateID:   data["permission_template_id"].(string),
			PermissionTemplateName: data["permission_template_name"].(string),
		})
	}

	response := &dto.RoleWithPermissionsResponse{
		ID:              role.ID,
		Name:            role.Name,
		Description:     role.Description,
		IsSystem:        role.IsSystem,
		Permissions:     permResponses,
		ModuleTemplates: moduleTemplates,
		CreatedAt:       role.CreatedAt,
		CreatedBy:       role.CreatedBy,
		UpdatedAt:       role.UpdatedAt,
		UpdatedBy:       role.UpdatedBy,
		DeletedAt:       role.DeletedAt,
		DeletedBy:       role.DeletedBy,
	}

	return response, nil
}

// AssignPermissions assigns permissions to a role
func (s *RoleService) AssignPermissions(roleID string, req *dto.AssignPermissionsRequest) error {
	logger.Info("Assigning permissions to role",
		logger.String("role_id", roleID),
		logger.Int("permission_count", len(req.PermissionIDs)),
	)

	// Check if role exists
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", roleID))
		return ErrRoleNotFound
	}

	// Check if it's a system role
	if role.IsSystem {
		logger.Warn("Cannot modify system role permissions", logger.String("role_id", roleID))
		return ErrSystemRoleCannotModify
	}

	// Assign permissions
	err = s.roleRepo.AssignPermissions(roleID, req.PermissionIDs)
	if err != nil {
		logger.Error("Failed to assign permissions", logger.Err(err))
		return err
	}

	logger.Info("Permissions assigned successfully", logger.String("role_id", roleID))
	return nil
}

// AddPermission adds a single permission to a role
func (s *RoleService) AddPermission(roleID string, permissionID string) error {
	logger.Info("Adding permission to role",
		logger.String("role_id", roleID),
		logger.String("permission_id", permissionID),
	)

	// Check if role exists
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", roleID))
		return ErrRoleNotFound
	}

	// Check if it's a system role
	if role.IsSystem {
		logger.Warn("Cannot modify system role permissions", logger.String("role_id", roleID))
		return ErrSystemRoleCannotModify
	}

	// Add permission
	err = s.roleRepo.AssignPermission(roleID, permissionID)
	if err != nil {
		logger.Error("Failed to add permission", logger.Err(err))
		return err
	}

	logger.Info("Permission added successfully", logger.String("role_id", roleID))
	return nil
}

// RemovePermission removes a single permission from a role
func (s *RoleService) RemovePermission(roleID string, permissionID string) error {
	logger.Info("Removing permission from role",
		logger.String("role_id", roleID),
		logger.String("permission_id", permissionID),
	)

	// Check if role exists
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", roleID))
		return ErrRoleNotFound
	}

	// Check if it's a system role
	if role.IsSystem {
		logger.Warn("Cannot modify system role permissions", logger.String("role_id", roleID))
		return ErrSystemRoleCannotModify
	}

	// Remove permission
	err = s.roleRepo.RemovePermission(roleID, permissionID)
	if err != nil {
		logger.Error("Failed to remove permission", logger.Err(err))
		return err
	}

	logger.Info("Permission removed successfully", logger.String("role_id", roleID))
	return nil
}

// Helper: Convert domain.Role to dto.RoleResponse
func (s *RoleService) toRoleResponse(role *domain.Role) dto.RoleResponse {
	return dto.RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedAt:   role.CreatedAt,
		CreatedBy:   role.CreatedBy,
		UpdatedAt:   role.UpdatedAt,
		UpdatedBy:   role.UpdatedBy,
		DeletedAt:   role.DeletedAt,
		DeletedBy:   role.DeletedBy,
	}
}

// expandTemplateToPermissions expands module templates to permission IDs
// moduleTemplates is a map of module_id -> permission_template_id
func (s *RoleService) expandTemplateToPermissions(moduleTemplates map[string]string) ([]string, error) {
	ctx := context.Background()
	var permissionIDs []string

	for moduleID, permissionTemplateID := range moduleTemplates {
		// Get actions from template
		query := `
			SELECT action
			FROM core.permission_template_actions
			WHERE permission_template_id = $1
		`
		rows, err := s.db.Query(ctx, query, permissionTemplateID)
		if err != nil {
			return nil, err
		}

		var actions []string
		for rows.Next() {
			var action string
			if err := rows.Scan(&action); err != nil {
				rows.Close()
				return nil, err
			}
			actions = append(actions, action)
		}
		rows.Close()

		// Get permission IDs for each module_id + action combination
		for _, action := range actions {
			permQuery := `
				SELECT id
				FROM core.permissions
				WHERE module_id = $1 AND action = $2 AND deleted_at IS NULL
			`
			var permID string
			err := s.db.QueryRow(ctx, permQuery, moduleID, action).Scan(&permID)
			if err == nil {
				permissionIDs = append(permissionIDs, permID)
			}
			// Ignore if permission doesn't exist for this module-action combination
		}
	}

	return permissionIDs, nil
}
