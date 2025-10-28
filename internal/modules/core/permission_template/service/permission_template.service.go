package service

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"lakukan-be/internal/modules/core/permission_template/domain"
	"lakukan-be/internal/modules/core/permission_template/dto"
	"lakukan-be/internal/modules/core/permission_template/repository"
)

var (
	ErrTemplateNotFound           = errors.New("permission template not found")
	ErrTemplateNameExists         = errors.New("permission template name already exists")
	ErrSystemTemplateCannotModify = errors.New("system template cannot be modified or deleted")
)

type PermissionTemplateService struct {
	templateRepo *repository.PermissionTemplateRepository
}

func NewPermissionTemplateService(templateRepo *repository.PermissionTemplateRepository) *PermissionTemplateService {
	return &PermissionTemplateService{
		templateRepo: templateRepo,
	}
}

// GetAll retrieves all permission templates with pagination
func (s *PermissionTemplateService) GetAll(params *dto.PermissionTemplateQueryParams) (*dto.PermissionTemplateListResponse, error) {
	offset := (params.Page - 1) * params.PageSize

	templates, err := s.templateRepo.FindAll(params.PageSize, offset, params.Search, params.IncludeSystem)
	if err != nil {
		return nil, err
	}

	total, err := s.templateRepo.Count(params.Search, params.IncludeSystem)
	if err != nil {
		return nil, err
	}

	// Get actions for each template
	var templateResponses []dto.PermissionTemplateResponse
	for _, template := range templates {
		actions, err := s.templateRepo.GetTemplateActions(template.ID)
		if err != nil {
			return nil, err
		}

		templateResponses = append(templateResponses, dto.PermissionTemplateResponse{
			ID:          template.ID,
			Name:        template.Name,
			Description: template.Description,
			IsSystem:    template.IsSystem,
			Actions:     actions,
			CreatedAt:   template.CreatedAt,
			CreatedBy:   template.CreatedBy,
			UpdatedAt:   template.UpdatedAt,
			UpdatedBy:   template.UpdatedBy,
			DeletedAt:   template.DeletedAt,
			DeletedBy:   template.DeletedBy,
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.PermissionTemplateListResponse{
		Templates:  templateResponses,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID retrieves a permission template by ID
func (s *PermissionTemplateService) GetByID(id string) (*dto.PermissionTemplateResponse, error) {
	template, err := s.templateRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrTemplateNotFound
	}

	actions, err := s.templateRepo.GetTemplateActions(id)
	if err != nil {
		return nil, err
	}

	return &dto.PermissionTemplateResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		IsSystem:    template.IsSystem,
		Actions:     actions,
		CreatedAt:   template.CreatedAt,
		CreatedBy:   template.CreatedBy,
		UpdatedAt:   template.UpdatedAt,
		UpdatedBy:   template.UpdatedBy,
		DeletedAt:   template.DeletedAt,
		DeletedBy:   template.DeletedBy,
	}, nil
}

// Create creates a new permission template with actions
func (s *PermissionTemplateService) Create(req *dto.CreatePermissionTemplateRequest, createdBy string) (*dto.PermissionTemplateResponse, error) {
	// Check if template name already exists
	existing, err := s.templateRepo.FindByName(req.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrTemplateNameExists
	}

	// Create template
	now := time.Now()
	template := &domain.PermissionTemplate{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false, // User-created templates are never system templates
		CreatedAt:   now,
		CreatedBy:   &createdBy,
		UpdatedAt:   now,
		UpdatedBy:   &createdBy,
	}

	if err := s.templateRepo.Create(template); err != nil {
		return nil, err
	}

	// Create template actions
	for _, action := range req.Actions {
		templateAction := &domain.PermissionTemplateAction{
			PermissionTemplateID: template.ID,
			Action:               action,
			CreatedAt:            now,
			CreatedBy:            &createdBy,
		}
		if err := s.templateRepo.CreateTemplateAction(templateAction); err != nil {
			return nil, err
		}
	}

	return &dto.PermissionTemplateResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		IsSystem:    template.IsSystem,
		Actions:     req.Actions,
		CreatedAt:   template.CreatedAt,
		CreatedBy:   template.CreatedBy,
		UpdatedAt:   template.UpdatedAt,
		UpdatedBy:   template.UpdatedBy,
	}, nil
}

// Update updates an existing permission template
func (s *PermissionTemplateService) Update(id string, req *dto.UpdatePermissionTemplateRequest, updatedBy string) (*dto.PermissionTemplateResponse, error) {
	// Get existing template
	template, err := s.templateRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrTemplateNotFound
	}

	// Cannot modify system templates
	if template.IsSystem {
		return nil, ErrSystemTemplateCannotModify
	}

	// Check if new name conflicts with existing template
	if template.Name != req.Name {
		existing, err := s.templateRepo.FindByName(req.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, ErrTemplateNameExists
		}
	}

	// Update template
	now := time.Now()
	template.Name = req.Name
	template.Description = req.Description
	template.UpdatedAt = now
	template.UpdatedBy = &updatedBy

	if err := s.templateRepo.Update(template); err != nil {
		return nil, err
	}

	// Delete old actions
	if err := s.templateRepo.DeleteTemplateActions(id); err != nil {
		return nil, err
	}

	// Create new actions
	for _, action := range req.Actions {
		templateAction := &domain.PermissionTemplateAction{
			PermissionTemplateID: id,
			Action:               action,
			CreatedAt:            now,
			CreatedBy:            &updatedBy,
		}
		if err := s.templateRepo.CreateTemplateAction(templateAction); err != nil {
			return nil, err
		}
	}

	return &dto.PermissionTemplateResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		IsSystem:    template.IsSystem,
		Actions:     req.Actions,
		CreatedAt:   template.CreatedAt,
		CreatedBy:   template.CreatedBy,
		UpdatedAt:   template.UpdatedAt,
		UpdatedBy:   template.UpdatedBy,
	}, nil
}

// Delete soft deletes a permission template
func (s *PermissionTemplateService) Delete(id string, deletedBy string) error {
	template, err := s.templateRepo.FindByID(id)
	if err != nil {
		return err
	}
	if template == nil {
		return ErrTemplateNotFound
	}

	// Cannot delete system templates
	if template.IsSystem {
		return ErrSystemTemplateCannotModify
	}

	return s.templateRepo.Delete(id, deletedBy)
}

// Restore restores a soft deleted permission template
func (s *PermissionTemplateService) Restore(id string) error {
	return s.templateRepo.Restore(id)
}
