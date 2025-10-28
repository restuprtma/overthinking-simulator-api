package service

import (
	"errors"

	"venturo-skeleton-go/internal/modules/core/permission/domain"
	"venturo-skeleton-go/internal/modules/core/permission/dto"
	"venturo-skeleton-go/internal/modules/core/permission/repository"
)

var (
	ErrModuleNotFound      = errors.New("module not found")
	ErrDuplicateModuleCode = errors.New("module with this code already exists")
)

type ModuleService struct {
	moduleRepo *repository.ModuleRepository
}

func NewModuleService(moduleRepo *repository.ModuleRepository) *ModuleService {
	return &ModuleService{
		moduleRepo: moduleRepo,
	}
}

// GetAll retrieves all modules (no pagination)
func (s *ModuleService) GetAll(params *dto.ModuleQueryParams) (*dto.ModuleListResponse, error) {
	// Get all modules
	modules, err := s.moduleRepo.FindAll(params.Search, params.IsActive)
	if err != nil {
		return nil, err
	}

	// Convert to response DTOs
	moduleResponses := make([]dto.ModuleDetailResponse, len(modules))
	for i, m := range modules {
		moduleResponses[i] = dto.ModuleDetailResponse{
			ID:          m.ID,
			Code:        m.Code,
			Name:        m.Name,
			Description: m.Description,
			Icon:        m.Icon,
			Color:       m.Color,
			SortOrder:   m.SortOrder,
			IsActive:    m.IsActive,

			// Tree structure fields
			ParentID: m.ParentID,
			Depth:    m.Depth,

			// Frontend-matching fields
			Heading:       m.Heading,
			To:            m.To,
			URL:           m.URL,
			Permission:    m.Permission,
			Tooltip:       m.Tooltip,
			IsMenuVisible: m.IsMenuVisible,

			// Related data counts
			PermissionsCount: m.PermissionsCount,

			// Audit fields
			CreatedAt: m.CreatedAt,
			CreatedBy: m.CreatedBy,
			UpdatedAt: m.UpdatedAt,
			UpdatedBy: m.UpdatedBy,
			DeletedAt: m.DeletedAt,
			DeletedBy: m.DeletedBy,
		}
	}

	total := int64(len(modules))

	return &dto.ModuleListResponse{
		Modules:    moduleResponses,
		Total:      total,
		Page:       1,
		PageSize:   int(total),
		TotalPages: 1,
	}, nil
}

// GetByID retrieves a module by ID
func (s *ModuleService) GetByID(id string) (*dto.ModuleDetailResponse, error) {
	module, err := s.moduleRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if module == nil {
		return nil, ErrModuleNotFound
	}

	return &dto.ModuleDetailResponse{
		ID:          module.ID,
		Code:        module.Code,
		Name:        module.Name,
		Description: module.Description,
		Icon:        module.Icon,
		Color:       module.Color,
		SortOrder:   module.SortOrder,
		IsActive:    module.IsActive,

		// Tree structure fields
		ParentID: module.ParentID,
		Depth:    module.Depth,

		// Frontend-matching fields
		Heading:       module.Heading,
		To:            module.To,
		URL:           module.URL,
		Permission:    module.Permission,
		Tooltip:       module.Tooltip,
		IsMenuVisible: module.IsMenuVisible,

		// Related data counts
		PermissionsCount: module.PermissionsCount,

		// Audit fields
		CreatedAt: module.CreatedAt,
		CreatedBy: module.CreatedBy,
		UpdatedAt: module.UpdatedAt,
		UpdatedBy: module.UpdatedBy,
		DeletedAt: module.DeletedAt,
		DeletedBy: module.DeletedBy,
	}, nil
}

// Create creates a new module
func (s *ModuleService) Create(req *dto.CreateModuleRequest, createdBy string) (*dto.ModuleDetailResponse, error) {
	// Check for duplicate code
	exists, err := s.moduleRepo.CheckDuplicateCode(req.Code, nil)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDuplicateModuleCode
	}

	// Set default values
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	isMenuVisible := true
	if req.IsMenuVisible != nil {
		isMenuVisible = *req.IsMenuVisible
	}

	// Calculate depth based on parent
	depth := s.calculateDepth(req.ParentID)

	// Create module domain model
	module := &domain.Module{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Color:       req.Color,
		SortOrder:   req.SortOrder,
		IsActive:    isActive,

		// Tree structure fields
		ParentID: req.ParentID,
		Depth:    depth,

		// Frontend-matching fields
		Heading:       req.Heading,
		To:            req.To,
		URL:           req.URL,
		Permission:    req.Permission,
		Tooltip:       req.Tooltip,
		IsMenuVisible: isMenuVisible,

		// Audit fields
		CreatedBy: &createdBy,
		UpdatedBy: &createdBy,
	}

	// Create module
	err = s.moduleRepo.Create(module)
	if err != nil {
		return nil, err
	}

	// Fetch the created module to get permissions_count
	createdModule, err := s.moduleRepo.FindByID(module.ID)
	if err != nil {
		return nil, err
	}

	return &dto.ModuleDetailResponse{
		ID:          createdModule.ID,
		Code:        createdModule.Code,
		Name:        createdModule.Name,
		Description: createdModule.Description,
		Icon:        createdModule.Icon,
		Color:       createdModule.Color,
		SortOrder:   createdModule.SortOrder,
		IsActive:    createdModule.IsActive,

		// Tree structure fields
		ParentID: createdModule.ParentID,
		Depth:    createdModule.Depth,

		// Frontend-matching fields
		Heading:       createdModule.Heading,
		To:            createdModule.To,
		URL:           createdModule.URL,
		Permission:    createdModule.Permission,
		Tooltip:       createdModule.Tooltip,
		IsMenuVisible: createdModule.IsMenuVisible,

		// Related data counts
		PermissionsCount: createdModule.PermissionsCount,

		// Audit fields
		CreatedAt: createdModule.CreatedAt,
		CreatedBy: createdModule.CreatedBy,
		UpdatedAt: createdModule.UpdatedAt,
		UpdatedBy: createdModule.UpdatedBy,
	}, nil
}

// Update updates a module
func (s *ModuleService) Update(id string, req *dto.UpdateModuleRequest, updatedBy string) (*dto.ModuleDetailResponse, error) {
	// Check if module exists
	existing, err := s.moduleRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, ErrModuleNotFound
	}

	// Apply updates
	if req.Code != nil {
		existing.Code = *req.Code
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Icon != nil {
		existing.Icon = req.Icon
	}
	if req.Color != nil {
		existing.Color = req.Color
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	// Tree structure updates
	if req.ParentID != nil {
		existing.ParentID = req.ParentID
		// Recalculate depth based on new parent
		existing.Depth = s.calculateDepth(req.ParentID)
	}

	// Frontend-matching field updates
	if req.Heading != nil {
		existing.Heading = req.Heading
	}
	if req.To != nil {
		existing.To = req.To
	}
	if req.URL != nil {
		existing.URL = req.URL
	}
	if req.Permission != nil {
		existing.Permission = req.Permission
	}
	if req.Tooltip != nil {
		existing.Tooltip = req.Tooltip
	}
	if req.IsMenuVisible != nil {
		existing.IsMenuVisible = *req.IsMenuVisible
	}

	existing.UpdatedBy = &updatedBy

	// Check for duplicate code
	exists, err := s.moduleRepo.CheckDuplicateCode(existing.Code, &id)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDuplicateModuleCode
	}

	// Update module
	err = s.moduleRepo.Update(existing)
	if err != nil {
		return nil, err
	}

	// Fetch updated module to get latest permissions_count
	updatedModule, err := s.moduleRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return &dto.ModuleDetailResponse{
		ID:          updatedModule.ID,
		Code:        updatedModule.Code,
		Name:        updatedModule.Name,
		Description: updatedModule.Description,
		Icon:        updatedModule.Icon,
		Color:       updatedModule.Color,
		SortOrder:   updatedModule.SortOrder,
		IsActive:    updatedModule.IsActive,

		// Tree structure fields
		ParentID: updatedModule.ParentID,
		Depth:    updatedModule.Depth,

		// Frontend-matching fields
		Heading:       updatedModule.Heading,
		To:            updatedModule.To,
		URL:           updatedModule.URL,
		Permission:    updatedModule.Permission,
		Tooltip:       updatedModule.Tooltip,
		IsMenuVisible: updatedModule.IsMenuVisible,

		// Related data counts
		PermissionsCount: updatedModule.PermissionsCount,

		// Audit fields
		CreatedAt: updatedModule.CreatedAt,
		CreatedBy: updatedModule.CreatedBy,
		UpdatedAt: updatedModule.UpdatedAt,
		UpdatedBy: updatedModule.UpdatedBy,
	}, nil
}

// Delete soft deletes a module
func (s *ModuleService) Delete(id string, deletedBy string) error {
	// Check if module exists
	module, err := s.moduleRepo.FindByID(id)
	if err != nil {
		return err
	}

	if module == nil {
		return ErrModuleNotFound
	}

	// Delete module
	err = s.moduleRepo.Delete(id, deletedBy)
	if err != nil {
		return err
	}

	return nil
}

// Restore restores a soft-deleted module
func (s *ModuleService) Restore(id string) error {
	// Restore module
	err := s.moduleRepo.Restore(id)
	if err != nil {
		return err
	}

	return nil
}

// GetTree retrieves modules as a tree structure grouped by root modules
func (s *ModuleService) GetTree() (dto.ModuleTreeResponse, error) {
	// Get all active modules (already ordered by recursive CTE: root_sort_order, path, sort_order)
	modules, err := s.moduleRepo.FindAll("", boolPtr(true))
	if err != nil {
		return nil, err
	}

	// Build module index and find root parent for each module
	moduleIndex := make(map[string]*domain.Module)
	for i := range modules {
		moduleIndex[modules[i].ID] = &modules[i]
	}

	// Separate root modules and group sub-modules by their root parent
	var rootModules []dto.ModuleDetailResponse
	subModulesMap := make(map[string][]dto.ModuleDetailResponse) // map[root_module_id][]all_descendants
	displayOrderCounter := make(map[string]int)                  // track display_order per root module

	for _, m := range modules {
		moduleDTO := dto.ModuleDetailResponse{
			ID:          m.ID,
			Code:        m.Code,
			Name:        m.Name,
			Description: m.Description,
			Icon:        m.Icon,
			Color:       m.Color,
			SortOrder:   m.SortOrder,
			IsActive:    m.IsActive,

			// Tree structure fields
			ParentID: m.ParentID,
			Depth:    m.Depth,

			// Frontend-matching fields
			Heading:       m.Heading,
			To:            m.To,
			URL:           m.URL,
			Permission:    m.Permission,
			Tooltip:       m.Tooltip,
			IsMenuVisible: m.IsMenuVisible,

			// Related data counts
			PermissionsCount: m.PermissionsCount,

			// Audit fields
			CreatedAt: m.CreatedAt,
			CreatedBy: m.CreatedBy,
			UpdatedAt: m.UpdatedAt,
			UpdatedBy: m.UpdatedBy,
			DeletedAt: m.DeletedAt,
			DeletedBy: m.DeletedBy,
		}

		if m.Depth == 0 {
			// Root module
			rootModules = append(rootModules, moduleDTO)
		} else {
			// Sub-module - find root parent and group under it
			rootParentID := findRootParent(&m, moduleIndex)
			if rootParentID != "" {
				// Assign display_order based on position in query result
				order := displayOrderCounter[rootParentID]
				moduleDTO.DisplayOrder = &order
				displayOrderCounter[rootParentID]++

				subModulesMap[rootParentID] = append(subModulesMap[rootParentID], moduleDTO)
			}
		}
	}

	// Build final response: group sub-modules under their root parent modules
	var result dto.ModuleTreeResponse
	for _, rootModule := range rootModules {
		subModules := subModulesMap[rootModule.ID]
		if subModules == nil {
			subModules = []dto.ModuleDetailResponse{} // Empty array instead of null
		}

		// Sort sub-modules: maintain hierarchical order from database query
		// The order should already be correct from the recursive CTE query,
		// but since we're grouping them in a map, we preserve the original order
		// by keeping them in the order they were added to the map

		result = append(result, dto.ModuleWithSubModules{
			Module:     rootModule,
			SubModules: subModules,
		})
	}

	return result, nil
}

// findRootParent recursively finds the root parent (depth=0) of a module
func findRootParent(module *domain.Module, moduleIndex map[string]*domain.Module) string {
	if module.ParentID == nil {
		return "" // No parent
	}

	parent, exists := moduleIndex[*module.ParentID]
	if !exists {
		return "" // Parent not found
	}

	if parent.Depth == 0 {
		return parent.ID // Found root parent
	}

	// Recursively find root parent
	return findRootParent(parent, moduleIndex)
}

// calculateDepth calculates the depth of a module based on its parent
// Returns 0 for root modules (no parent), or parent depth + 1 for child modules
func (s *ModuleService) calculateDepth(parentID *string) int {
	if parentID == nil {
		return 0 // Root module
	}

	parent, err := s.moduleRepo.FindByID(*parentID)
	if err != nil || parent == nil {
		return 0 // Fallback to root if parent not found
	}

	return parent.Depth + 1
}

// boolPtr is a helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
