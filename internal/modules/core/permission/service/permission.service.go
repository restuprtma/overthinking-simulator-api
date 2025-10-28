package service

import (
	"errors"
	"math"

	"lakukan-be/internal/modules/core/permission/domain"
	"lakukan-be/internal/modules/core/permission/dto"
	"lakukan-be/internal/modules/core/permission/repository"
)

var (
	ErrPermissionNotFound      = errors.New("permission not found")
	ErrDuplicateResourceAction = errors.New("permission with this resource and action already exists")
	ErrPermissionStillInUse    = errors.New("permission is still assigned to roles")
)

type PermissionService struct {
	permissionRepo *repository.PermissionRepository
}

func NewPermissionService(permissionRepo *repository.PermissionRepository) *PermissionService {
	return &PermissionService{
		permissionRepo: permissionRepo,
	}
}

// GetAll retrieves all permissions with pagination
func (s *PermissionService) GetAll(params *dto.PermissionQueryParams) (*dto.PermissionListResponse, error) {
	// Calculate pagination
	limit := params.PageSize
	offset := (params.Page - 1) * params.PageSize

	// Get permissions
	permissions, err := s.permissionRepo.FindAll(limit, offset, params.Search, params.ModuleID, params.Resource, params.Action)
	if err != nil {
		return nil, err
	}

	// Get total count
	total, err := s.permissionRepo.Count(params.Search, params.ModuleID, params.Resource, params.Action)
	if err != nil {
		return nil, err
	}

	// Convert to response DTOs
	permissionResponses := make([]dto.PermissionResponse, len(permissions))
	for i, p := range permissions {
		moduleResp := convertModuleDomainToDTO(p.Module)
		permissionResponses[i] = dto.PermissionResponse{
			ID:          p.ID,
			ModuleID:    p.ModuleID,
			Module:      moduleResp,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			CreatedBy:   p.CreatedBy,
			UpdatedAt:   p.UpdatedAt,
			UpdatedBy:   p.UpdatedBy,
			DeletedAt:   p.DeletedAt,
			DeletedBy:   p.DeletedBy,
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.PermissionListResponse{
		Permissions: permissionResponses,
		Total:       total,
		Page:        params.Page,
		PageSize:    params.PageSize,
		TotalPages:  totalPages,
	}, nil
}

// GetByID retrieves a permission by ID
func (s *PermissionService) GetByID(id string) (*dto.PermissionResponse, error) {
	permission, err := s.permissionRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if permission == nil {
		return nil, ErrPermissionNotFound
	}

	moduleResp := convertModuleDomainToDTO(permission.Module)

	return &dto.PermissionResponse{
		ID:          permission.ID,
		ModuleID:    permission.ModuleID,
		Module:      moduleResp,
		Resource:    permission.Resource,
		Action:      permission.Action,
		Description: permission.Description,
		CreatedAt:   permission.CreatedAt,
		CreatedBy:   permission.CreatedBy,
		UpdatedAt:   permission.UpdatedAt,
		UpdatedBy:   permission.UpdatedBy,
		DeletedAt:   permission.DeletedAt,
		DeletedBy:   permission.DeletedBy,
	}, nil
}

// Create creates a new permission
func (s *PermissionService) Create(req *dto.CreatePermissionRequest, createdBy string) (*dto.PermissionResponse, error) {
	// Check for duplicate resource-action combination
	exists, err := s.permissionRepo.CheckDuplicateResourceAction(req.ModuleID, req.Resource, req.Action, nil)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDuplicateResourceAction
	}

	// Create permission domain model
	permission := &domain.Permission{
		ModuleID:    req.ModuleID,
		Resource:    req.Resource,
		Action:      req.Action,
		Description: req.Description,
		CreatedBy:   &createdBy,
		UpdatedBy:   &createdBy,
	}

	// Create permission
	err = s.permissionRepo.Create(permission)
	if err != nil {
		return nil, err
	}

	// Fetch the created permission with module info
	created, err := s.permissionRepo.FindByID(permission.ID)
	if err != nil {
		return nil, err
	}

	moduleResp := convertModuleDomainToDTO(created.Module)

	return &dto.PermissionResponse{
		ID:          created.ID,
		ModuleID:    created.ModuleID,
		Module:      moduleResp,
		Resource:    created.Resource,
		Action:      created.Action,
		Description: created.Description,
		CreatedAt:   created.CreatedAt,
		CreatedBy:   created.CreatedBy,
		UpdatedAt:   created.UpdatedAt,
		UpdatedBy:   created.UpdatedBy,
		DeletedAt:   created.DeletedAt,
		DeletedBy:   created.DeletedBy,
	}, nil
}

// Update updates a permission
func (s *PermissionService) Update(id string, req *dto.UpdatePermissionRequest, updatedBy string) (*dto.PermissionResponse, error) {
	// Check if permission exists
	existing, err := s.permissionRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, ErrPermissionNotFound
	}

	// Apply updates
	if req.ModuleID != nil {
		existing.ModuleID = *req.ModuleID
	}
	if req.Resource != nil {
		existing.Resource = *req.Resource
	}
	if req.Action != nil {
		existing.Action = *req.Action
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	existing.UpdatedBy = &updatedBy

	// Check for duplicates
	exists, err := s.permissionRepo.CheckDuplicateResourceAction(existing.ModuleID, existing.Resource, existing.Action, &id)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDuplicateResourceAction
	}

	// Update permission
	err = s.permissionRepo.Update(existing)
	if err != nil {
		return nil, err
	}

	// Fetch the updated permission with module info
	updated, err := s.permissionRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	moduleResp := convertModuleDomainToDTO(updated.Module)

	return &dto.PermissionResponse{
		ID:          updated.ID,
		ModuleID:    updated.ModuleID,
		Module:      moduleResp,
		Resource:    updated.Resource,
		Action:      updated.Action,
		Description: updated.Description,
		CreatedAt:   updated.CreatedAt,
		UpdatedAt:   updated.UpdatedAt,
	}, nil
}

// Delete soft deletes a permission
func (s *PermissionService) Delete(id string, deletedBy string) error {
	// Check if permission exists
	permission, err := s.permissionRepo.FindByID(id)
	if err != nil {
		return err
	}

	if permission == nil {
		return ErrPermissionNotFound
	}

	// Delete permission
	err = s.permissionRepo.Delete(id, deletedBy)
	if err != nil {
		return err
	}

	return nil
}

// GetActions retrieves all unique action names
func (s *PermissionService) GetActions() ([]string, error) {
	return s.permissionRepo.GetDistinctActions()
}

// ApplyCRUDTemplate creates standard CRUD permissions (create, read, update, delete) for a resource
func (s *PermissionService) ApplyCRUDTemplate(req *dto.ApplyCRUDTemplateRequest, createdBy string) (*dto.CRUDTemplateResponse, error) {
	standardActions := []string{"create", "read", "update", "delete"}
	var created []dto.PermissionResponse
	var skipped []string

	for _, action := range standardActions {
		// Check if permission already exists
		exists, err := s.permissionRepo.CheckDuplicateResourceAction(req.ModuleID, req.Resource, action, nil)
		if err != nil {
			return nil, err
		}

		if exists {
			skipped = append(skipped, action)
			continue
		}

		// Create permission
		description := getStandardPermissionDescription(req.Resource, action)
		permission := &domain.Permission{
			ModuleID:    req.ModuleID,
			Resource:    req.Resource,
			Action:      action,
			Description: &description,
			CreatedBy:   &createdBy,
			UpdatedBy:   &createdBy,
		}

		err = s.permissionRepo.Create(permission)
		if err != nil {
			return nil, err
		}

		// Fetch created permission to include module info
		createdPermission, err := s.permissionRepo.FindByID(permission.ID)
		if err != nil {
			return nil, err
		}

		moduleResp := convertModuleDomainToDTO(createdPermission.Module)
		created = append(created, dto.PermissionResponse{
			ID:          createdPermission.ID,
			ModuleID:    createdPermission.ModuleID,
			Module:      moduleResp,
			Resource:    createdPermission.Resource,
			Action:      createdPermission.Action,
			Description: createdPermission.Description,
			CreatedAt:   createdPermission.CreatedAt,
			CreatedBy:   createdPermission.CreatedBy,
			UpdatedAt:   createdPermission.UpdatedAt,
			UpdatedBy:   createdPermission.UpdatedBy,
		})
	}

	return &dto.CRUDTemplateResponse{
		Created: created,
		Skipped: skipped,
		Total:   len(created),
	}, nil
}

// Helper function to generate standard permission descriptions
func getStandardPermissionDescription(resource, action string) string {
	descriptions := map[string]string{
		"create": "Create new " + resource,
		"read":   "View " + resource,
		"update": "Update " + resource,
		"delete": "Delete " + resource,
	}
	if desc, ok := descriptions[action]; ok {
		return desc
	}
	return action + " " + resource
}

// Helper function to convert Module domain to DTO
func convertModuleDomainToDTO(module *domain.Module) *dto.ModuleResponse {
	if module == nil {
		return nil
	}
	return &dto.ModuleResponse{
		ID:          module.ID,
		Code:        module.Code,
		Name:        module.Name,
		Description: module.Description,
		Icon:        module.Icon,
		Color:       module.Color,
		SortOrder:   module.SortOrder,
		IsActive:    module.IsActive,
	}
}
