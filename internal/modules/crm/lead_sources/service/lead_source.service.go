package service

import (
	"errors"
	"math"
	"time"

	"venturo-skeleton-go/internal/modules/crm/lead_sources/domain"
	"venturo-skeleton-go/internal/modules/crm/lead_sources/dto"
	"venturo-skeleton-go/internal/modules/crm/lead_sources/repository"
	"venturo-skeleton-go/pkg/logger"

	"github.com/google/uuid"
)

var (
	ErrLeadSourceNotFound         = errors.New("lead source not found")
	ErrLeadSourceCodeAlreadyExists = errors.New("lead source code already exists")
)

type LeadSourceService struct {
	repo *repository.LeadSourceRepository
}

func NewLeadSourceService(repo *repository.LeadSourceRepository) *LeadSourceService {
	return &LeadSourceService{
		repo: repo,
	}
}

// GetAll gets all lead sources with pagination
func (s *LeadSourceService) GetAll(companyID string, params *dto.LeadSourceQueryParams) (*dto.LeadSourceListResponse, error) {
	logger.Info("Fetching lead sources list",
		logger.String("company_id", companyID),
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
		logger.String("search", params.Search),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get lead sources
	leadSources, err := s.repo.FindAll(companyID, params.PageSize, offset, params.Search, params.IsActive)
	if err != nil {
		logger.Error("Failed to fetch lead sources", logger.Err(err))
		return nil, err
	}

	// Get total count
	total, err := s.repo.Count(companyID, params.Search, params.IsActive)
	if err != nil {
		logger.Error("Failed to count lead sources", logger.Err(err))
		return nil, err
	}

	// Convert to response
	responses := make([]dto.LeadSourceResponse, len(leadSources))
	for i, ls := range leadSources {
		responses[i] = s.toResponse(&ls)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.LeadSourceListResponse{
		LeadSources: responses,
		Total:       total,
		Page:        params.Page,
		PageSize:    params.PageSize,
		TotalPages:  totalPages,
	}, nil
}

// GetByID gets a lead source by ID
func (s *LeadSourceService) GetByID(id, companyID string) (*dto.LeadSourceResponse, error) {
	logger.Info("Fetching lead source by ID",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	ls, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead source", logger.Err(err))
		return nil, err
	}

	if ls == nil {
		logger.Warn("Lead source not found", logger.String("id", id))
		return nil, ErrLeadSourceNotFound
	}

	response := s.toResponse(ls)
	return &response, nil
}

// Create creates a new lead source
func (s *LeadSourceService) Create(companyID, userID string, req *dto.CreateLeadSourceRequest) (*dto.LeadSourceResponse, error) {
	logger.Info("Creating new lead source",
		logger.String("company_id", companyID),
		logger.String("name", req.Name),
		logger.String("code", req.Code),
	)

	// Check if code already exists
	existing, err := s.repo.FindByCode(req.Code, companyID)
	if err != nil {
		logger.Error("Failed to check existing code", logger.Err(err))
		return nil, err
	}
	if existing != nil {
		logger.Warn("Lead source code already exists",
			logger.String("code", req.Code),
			logger.String("company_id", companyID),
		)
		return nil, ErrLeadSourceCodeAlreadyExists
	}

	// Create domain entity
	now := time.Now()
	ls := &domain.LeadSource{
		ID:          uuid.New().String(),
		CompanyID:   companyID,
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		IsActive:    true,
		SortOrder:   0,
		CreatedAt:   now,
		CreatedBy:   &userID,
		UpdatedAt:   now,
		UpdatedBy:   &userID,
	}

	// Apply optional fields
	if req.IsActive != nil {
		ls.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		ls.SortOrder = *req.SortOrder
	}

	// Save to database
	if err := s.repo.Create(ls); err != nil {
		logger.Error("Failed to create lead source", logger.Err(err))
		return nil, err
	}

	logger.Info("Lead source created successfully", logger.String("id", ls.ID))

	response := s.toResponse(ls)
	return &response, nil
}

// Update updates an existing lead source
func (s *LeadSourceService) Update(id, companyID, userID string, req *dto.UpdateLeadSourceRequest) (*dto.LeadSourceResponse, error) {
	logger.Info("Updating lead source",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Get existing lead source
	ls, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead source", logger.Err(err))
		return nil, err
	}
	if ls == nil {
		logger.Warn("Lead source not found", logger.String("id", id))
		return nil, ErrLeadSourceNotFound
	}

	// Check if code is being changed and already exists
	if req.Code != nil && *req.Code != ls.Code {
		existing, err := s.repo.FindByCode(*req.Code, companyID)
		if err != nil {
			logger.Error("Failed to check existing code", logger.Err(err))
			return nil, err
		}
		if existing != nil {
			logger.Warn("Lead source code already exists", logger.String("code", *req.Code))
			return nil, ErrLeadSourceCodeAlreadyExists
		}
		ls.Code = *req.Code
	}

	// Update fields
	if req.Name != nil {
		ls.Name = *req.Name
	}
	if req.Description != nil {
		ls.Description = req.Description
	}
	if req.Color != nil {
		ls.Color = req.Color
	}
	if req.Icon != nil {
		ls.Icon = req.Icon
	}
	if req.IsActive != nil {
		ls.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		ls.SortOrder = *req.SortOrder
	}

	ls.UpdatedAt = time.Now()
	ls.UpdatedBy = &userID

	// Save changes
	if err := s.repo.Update(ls); err != nil {
		logger.Error("Failed to update lead source", logger.Err(err))
		return nil, err
	}

	logger.Info("Lead source updated successfully", logger.String("id", id))

	response := s.toResponse(ls)
	return &response, nil
}

// Delete soft deletes a lead source
func (s *LeadSourceService) Delete(id, companyID, userID string) error {
	logger.Info("Deleting lead source",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Check if exists
	ls, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead source", logger.Err(err))
		return err
	}
	if ls == nil {
		logger.Warn("Lead source not found", logger.String("id", id))
		return ErrLeadSourceNotFound
	}

	// Soft delete
	if err := s.repo.Delete(id, companyID, userID); err != nil {
		logger.Error("Failed to delete lead source", logger.Err(err))
		return err
	}

	logger.Info("Lead source deleted successfully", logger.String("id", id))
	return nil
}

// Restore restores a soft-deleted lead source
func (s *LeadSourceService) Restore(id, companyID string) error {
	logger.Info("Restoring lead source",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	if err := s.repo.Restore(id, companyID); err != nil {
		logger.Error("Failed to restore lead source", logger.Err(err))
		return err
	}

	logger.Info("Lead source restored successfully", logger.String("id", id))
	return nil
}

// toResponse converts domain to response DTO
func (s *LeadSourceService) toResponse(ls *domain.LeadSource) dto.LeadSourceResponse {
	return dto.LeadSourceResponse{
		ID:          ls.ID,
		CompanyID:   ls.CompanyID,
		Name:        ls.Name,
		Code:        ls.Code,
		Description: ls.Description,
		Color:       ls.Color,
		Icon:        ls.Icon,
		IsActive:    ls.IsActive,
		SortOrder:   ls.SortOrder,
		CreatedAt:   ls.CreatedAt,
		UpdatedAt:   ls.UpdatedAt,
		DeletedAt:   ls.DeletedAt,
	}
}
