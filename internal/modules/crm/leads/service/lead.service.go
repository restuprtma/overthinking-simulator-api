package service

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"lakukan-be/internal/modules/crm/leads/domain"
	"lakukan-be/internal/modules/crm/leads/dto"
	"lakukan-be/internal/modules/crm/leads/repository"
	"lakukan-be/pkg/logger"
)

var (
	ErrLeadNotFound         = errors.New("lead not found")
	ErrPhoneAlreadyExists   = errors.New("phone number already exists in this company")
)

type LeadService struct {
	repo *repository.LeadRepository
}

func NewLeadService(repo *repository.LeadRepository) *LeadService {
	return &LeadService{repo: repo}
}

// GetAll returns paginated list of leads
func (s *LeadService) GetAll(companyID string, params dto.LeadQueryParams) ([]dto.LeadResponse, int, int, error) {
	logger.Info("Fetching leads",
		logger.String("company_id", companyID),
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get total count
	total, err := s.repo.Count(companyID, params.Search, params.Category, params.Status, params.SourceID, params.AssignedToCompanyUserID)
	if err != nil {
		logger.Error("Failed to count leads", logger.Err(err))
		return nil, 0, 0, err
	}

	// Get leads
	leads, err := s.repo.FindAll(companyID, params.PageSize, offset, params.Search, params.Category, params.Status, params.SourceID, params.AssignedToCompanyUserID)
	if err != nil {
		logger.Error("Failed to fetch leads", logger.Err(err))
		return nil, 0, 0, err
	}

	// Convert to response DTOs
	responses := make([]dto.LeadResponse, len(leads))
	for i, lead := range leads {
		responses[i] = s.toResponse(lead)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	logger.Info("Successfully fetched leads",
		logger.Int("total", total),
		logger.Int("count", len(responses)),
	)

	return responses, total, totalPages, nil
}

// GetByID returns a single lead by ID
func (s *LeadService) GetByID(id, companyID string) (*dto.LeadResponse, error) {
	logger.Info("Fetching lead by ID",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	lead, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead", logger.Err(err))
		return nil, err
	}

	if lead == nil {
		logger.Warn("Lead not found",
			logger.String("id", id),
			logger.String("company_id", companyID),
		)
		return nil, ErrLeadNotFound
	}

	response := s.toResponse(lead)
	logger.Info("Successfully fetched lead", logger.String("id", id))

	return &response, nil
}

// Create creates a new lead
func (s *LeadService) Create(companyID, userID string, req dto.CreateLeadRequest) (*dto.LeadResponse, error) {
	logger.Info("Creating new lead",
		logger.String("company_id", companyID),
		logger.String("phone", req.Phone),
		logger.String("name", req.Name),
	)

	// Check if phone already exists
	existing, err := s.repo.FindByPhone(req.Phone, companyID)
	if err != nil {
		logger.Error("Failed to check existing phone", logger.Err(err))
		return nil, err
	}
	if existing != nil {
		logger.Warn("Phone already exists",
			logger.String("phone", req.Phone),
			logger.String("company_id", companyID),
		)
		return nil, ErrPhoneAlreadyExists
	}

	// Generate lead number
	leadNumber := s.generateLeadNumber(companyID)

	// Default status
	status := "new"
	if req.Status != nil {
		status = *req.Status
	}

	// Create lead entity
	lead := &domain.Lead{
		ID:                     uuid.New().String(),
		CompanyID:              companyID,
		LeadNumber:             &leadNumber,
		Name:                   req.Name,
		ContactPerson:          req.ContactPerson,
		Phone:                  req.Phone,
		Email:                  req.Email,
		Category:               req.Category,
		Status:                 status,
		SourceID:               req.SourceID,
		AssignedToCompanyUserID: req.AssignedToCompanyUserID,
		DealValue:              req.DealValue,
		NextFollowUpAt:         req.NextFollowUpAt,
		Notes:                  req.Notes,
		Address:                req.Address,
		City:                   req.City,
		Province:               req.Province,
		PostalCode:             req.PostalCode,
		CreatedAt:              time.Now(),
		CreatedBy:              &userID,
		UpdatedAt:              time.Now(),
		UpdatedBy:              &userID,
	}

	// Save to database
	if err := s.repo.Create(lead); err != nil {
		logger.Error("Failed to create lead", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully created lead",
		logger.String("id", lead.ID),
		logger.String("lead_number", *lead.LeadNumber),
	)

	// Fetch created lead with joined fields
	created, err := s.repo.FindByID(lead.ID, companyID)
	if err != nil {
		logger.Error("Failed to fetch created lead", logger.Err(err))
		return nil, err
	}

	response := s.toResponse(created)
	return &response, nil
}

// Update updates an existing lead
func (s *LeadService) Update(id, companyID, userID string, req dto.UpdateLeadRequest) (*dto.LeadResponse, error) {
	logger.Info("Updating lead",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Check if lead exists
	lead, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead", logger.Err(err))
		return nil, err
	}
	if lead == nil {
		logger.Warn("Lead not found", logger.String("id", id))
		return nil, ErrLeadNotFound
	}

	// Check if new phone already exists (if changed)
	if req.Phone != nil && *req.Phone != lead.Phone {
		existing, err := s.repo.FindByPhone(*req.Phone, companyID)
		if err != nil {
			logger.Error("Failed to check existing phone", logger.Err(err))
			return nil, err
		}
		if existing != nil && existing.ID != id {
			logger.Warn("Phone already exists",
				logger.String("phone", *req.Phone),
				logger.String("company_id", companyID),
			)
			return nil, ErrPhoneAlreadyExists
		}
		lead.Phone = *req.Phone
	}

	// Update fields
	if req.Name != nil {
		lead.Name = *req.Name
	}
	if req.ContactPerson != nil {
		lead.ContactPerson = req.ContactPerson
	}
	if req.Email != nil {
		lead.Email = req.Email
	}
	if req.Category != nil {
		lead.Category = *req.Category
	}
	if req.Status != nil {
		lead.Status = *req.Status
	}
	if req.SourceID != nil {
		lead.SourceID = *req.SourceID
	}
	if req.AssignedToCompanyUserID != nil {
		lead.AssignedToCompanyUserID = req.AssignedToCompanyUserID
	}
	if req.DealValue != nil {
		lead.DealValue = req.DealValue
	}
	if req.LastContactAt != nil {
		lead.LastContactAt = req.LastContactAt
	}
	if req.NextFollowUpAt != nil {
		lead.NextFollowUpAt = req.NextFollowUpAt
	}
	if req.Notes != nil {
		lead.Notes = req.Notes
	}
	if req.Address != nil {
		lead.Address = req.Address
	}
	if req.City != nil {
		lead.City = req.City
	}
	if req.Province != nil {
		lead.Province = req.Province
	}
	if req.PostalCode != nil {
		lead.PostalCode = req.PostalCode
	}

	lead.UpdatedAt = time.Now()
	lead.UpdatedBy = &userID

	// Save to database
	if err := s.repo.Update(lead); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Lead not found during update", logger.String("id", id))
			return nil, ErrLeadNotFound
		}
		logger.Error("Failed to update lead", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully updated lead", logger.String("id", id))

	// Fetch updated lead
	updated, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch updated lead", logger.Err(err))
		return nil, err
	}

	response := s.toResponse(updated)
	return &response, nil
}

// Delete soft deletes a lead
func (s *LeadService) Delete(id, companyID, userID string) error {
	logger.Info("Deleting lead",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Check if lead exists
	lead, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch lead", logger.Err(err))
		return err
	}
	if lead == nil {
		logger.Warn("Lead not found", logger.String("id", id))
		return ErrLeadNotFound
	}

	// Soft delete
	if err := s.repo.Delete(id, companyID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Lead not found during delete", logger.String("id", id))
			return ErrLeadNotFound
		}
		logger.Error("Failed to delete lead", logger.Err(err))
		return err
	}

	logger.Info("Successfully deleted lead", logger.String("id", id))
	return nil
}

// Restore restores a soft-deleted lead
func (s *LeadService) Restore(id, companyID string) error {
	logger.Info("Restoring lead",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Restore
	if err := s.repo.Restore(id, companyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Lead not found or not deleted", logger.String("id", id))
			return ErrLeadNotFound
		}
		logger.Error("Failed to restore lead", logger.Err(err))
		return err
	}

	logger.Info("Successfully restored lead", logger.String("id", id))
	return nil
}

// generateLeadNumber generates a unique lead number
func (s *LeadService) generateLeadNumber(companyID string) string {
	// Format: LEAD-YYYYMMDD-XXXXX (e.g., LEAD-20250124-00001)
	now := time.Now()
	dateStr := now.Format("20060102")

	// In production, you might want to query the database for the last number of the day
	// and increment it. For simplicity, we'll use a timestamp-based approach
	timeStr := fmt.Sprintf("%05d", now.Unix()%100000)

	return fmt.Sprintf("LEAD-%s-%s", dateStr, timeStr)
}

// toResponse converts domain entity to response DTO
func (s *LeadService) toResponse(lead *domain.Lead) dto.LeadResponse {
	response := dto.LeadResponse{
		ID:                  lead.ID,
		CompanyID:           lead.CompanyID,
		LeadNumber:          lead.LeadNumber,
		Name:                lead.Name,
		ContactPerson:       lead.ContactPerson,
		Phone:               lead.Phone,
		Email:               lead.Email,
		Category:            lead.Category,
		Status:              lead.Status,
		DealValue:           lead.DealValue,
		LastContactAt:       lead.LastContactAt,
		NextFollowUpAt:      lead.NextFollowUpAt,
		ResponseTimeMinutes: lead.ResponseTimeMinutes,
		Notes:               lead.Notes,
		Address:             lead.Address,
		City:                lead.City,
		Province:            lead.Province,
		PostalCode:          lead.PostalCode,
	}

	// Build source info
	if lead.SourceName != nil {
		response.Source = &dto.LeadSourceInfo{
			ID:   lead.SourceID,
			Name: *lead.SourceName,
			Code: "", // Code is not loaded in the query for performance
		}
	}

	// Build assigned user info
	if lead.AssignedToCompanyUserID != nil && lead.AssignedToUserID != nil && lead.AssignedToUserName != nil && lead.AssignedToUserEmail != nil {
		response.AssignedTo = &dto.AssignedUserInfo{
			CompanyUserID: *lead.AssignedToCompanyUserID,
			UserID:        *lead.AssignedToUserID,
			FullName:      *lead.AssignedToUserName,
			Email:         *lead.AssignedToUserEmail,
		}
	}

	// Build audit info
	response.Audit = &dto.AuditInfo{
		CreatedAt:     lead.CreatedAt,
		CreatedBy:     lead.CreatedBy,
		CreatedByName: lead.CreatedByName,
		UpdatedAt:     lead.UpdatedAt,
		UpdatedBy:     lead.UpdatedBy,
		UpdatedByName: lead.UpdatedByName,
		DeletedAt:     lead.DeletedAt,
		DeletedBy:     lead.DeletedBy,
		DeletedByName: lead.DeletedByName,
	}

	return response
}
