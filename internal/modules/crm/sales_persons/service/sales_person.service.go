package service

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"lakukan-be/internal/modules/crm/sales_persons/domain"
	"lakukan-be/internal/modules/crm/sales_persons/dto"
	"lakukan-be/internal/modules/crm/sales_persons/repository"
	"lakukan-be/pkg/logger"
)

var (
	ErrSalesPersonNotFound              = errors.New("sales person not found")
	ErrSalesCodeAlreadyExists           = errors.New("sales code already exists in this company")
	ErrCompanyUserAlreadyHasSalesPerson = errors.New("this company user already has a sales person record")
)

type SalesPersonService struct {
	repo *repository.SalesPersonRepository
}

func NewSalesPersonService(repo *repository.SalesPersonRepository) *SalesPersonService {
	return &SalesPersonService{repo: repo}
}

// GetAll returns paginated list of sales persons
func (s *SalesPersonService) GetAll(companyID string, params dto.SalesPersonQueryParams) ([]dto.SalesPersonResponse, int, int, error) {
	logger.Info("Fetching sales persons",
		logger.String("company_id", companyID),
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get total count
	total, err := s.repo.Count(companyID, params.Search, params.IsActive, params.IsWhatsappConnected, params.SalesArea)
	if err != nil {
		logger.Error("Failed to count sales persons", logger.Err(err))
		return nil, 0, 0, err
	}

	// Get sales persons
	salesPersons, err := s.repo.FindAll(companyID, params.PageSize, offset, params.Search, params.IsActive, params.IsWhatsappConnected, params.SalesArea)
	if err != nil {
		logger.Error("Failed to fetch sales persons", logger.Err(err))
		return nil, 0, 0, err
	}

	// Convert to response DTOs
	responses := make([]dto.SalesPersonResponse, len(salesPersons))
	for i, sp := range salesPersons {
		responses[i] = s.toResponse(sp)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	logger.Info("Successfully fetched sales persons",
		logger.Int("total", total),
		logger.Int("count", len(responses)),
	)

	return responses, total, totalPages, nil
}

// GetByID returns a single sales person by ID
func (s *SalesPersonService) GetByID(id, companyID string) (*dto.SalesPersonResponse, error) {
	logger.Info("Fetching sales person by ID",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	sp, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch sales person", logger.Err(err))
		return nil, err
	}

	if sp == nil {
		logger.Warn("Sales person not found",
			logger.String("id", id),
			logger.String("company_id", companyID),
		)
		return nil, ErrSalesPersonNotFound
	}

	response := s.toResponse(sp)
	logger.Info("Successfully fetched sales person", logger.String("id", id))

	return &response, nil
}

// Create creates a new sales person
func (s *SalesPersonService) Create(companyID, userID string, req dto.CreateSalesPersonRequest) (*dto.SalesPersonResponse, error) {
	logger.Info("Creating new sales person",
		logger.String("company_id", companyID),
		logger.String("sales_code", req.SalesCode),
		logger.String("company_user_id", req.CompanyUserID),
	)

	// Check if sales code already exists
	existing, err := s.repo.FindBySalesCode(req.SalesCode, companyID)
	if err != nil {
		logger.Error("Failed to check existing sales code", logger.Err(err))
		return nil, err
	}
	if existing != nil {
		logger.Warn("Sales code already exists",
			logger.String("sales_code", req.SalesCode),
			logger.String("company_id", companyID),
		)
		return nil, ErrSalesCodeAlreadyExists
	}

	// Check if company_user_id already has a sales person record
	existingByCU, err := s.repo.FindByCompanyUserID(req.CompanyUserID, companyID)
	if err != nil {
		logger.Error("Failed to check existing company user", logger.Err(err))
		return nil, err
	}
	if existingByCU != nil {
		logger.Warn("Company user already has a sales person record",
			logger.String("company_user_id", req.CompanyUserID),
			logger.String("company_id", companyID),
		)
		return nil, ErrCompanyUserAlreadyHasSalesPerson
	}

	// Create sales person entity
	sp := &domain.SalesPerson{
		ID:                  uuid.New().String(),
		CompanyUserID:       req.CompanyUserID,
		SalesCode:           req.SalesCode,
		SalesName:           req.SalesName,
		SalesArea:           req.SalesArea,
		SalesTarget:         req.SalesTarget,
		CommissionRate:      req.CommissionRate,
		Whatsapp:            req.Whatsapp,
		IsWhatsappConnected: false, // Default to false
		IsActive:            true,  // Default to true
		Notes:               req.Notes,
		CreatedAt:           time.Now(),
		CreatedBy:           &userID,
		UpdatedAt:           time.Now(),
		UpdatedBy:           &userID,
	}

	// Override defaults if provided
	if req.IsWhatsappConnected != nil {
		sp.IsWhatsappConnected = *req.IsWhatsappConnected
	}
	if req.IsActive != nil {
		sp.IsActive = *req.IsActive
	}

	// Save to database
	if err := s.repo.Create(sp); err != nil {
		logger.Error("Failed to create sales person", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully created sales person",
		logger.String("id", sp.ID),
		logger.String("sales_code", sp.SalesCode),
	)

	// Fetch created sales person with joined fields
	created, err := s.repo.FindByID(sp.ID, companyID)
	if err != nil {
		logger.Error("Failed to fetch created sales person", logger.Err(err))
		return nil, err
	}

	response := s.toResponse(created)
	return &response, nil
}

// Update updates an existing sales person
func (s *SalesPersonService) Update(id, companyID, userID string, req dto.UpdateSalesPersonRequest) (*dto.SalesPersonResponse, error) {
	logger.Info("Updating sales person",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Check if sales person exists
	sp, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch sales person", logger.Err(err))
		return nil, err
	}
	if sp == nil {
		logger.Warn("Sales person not found", logger.String("id", id))
		return nil, ErrSalesPersonNotFound
	}

	// Check if new sales code already exists (if changed)
	if req.SalesCode != nil && *req.SalesCode != sp.SalesCode {
		existing, err := s.repo.FindBySalesCode(*req.SalesCode, companyID)
		if err != nil {
			logger.Error("Failed to check existing sales code", logger.Err(err))
			return nil, err
		}
		if existing != nil && existing.ID != id {
			logger.Warn("Sales code already exists",
				logger.String("sales_code", *req.SalesCode),
				logger.String("company_id", companyID),
			)
			return nil, ErrSalesCodeAlreadyExists
		}
		sp.SalesCode = *req.SalesCode
	}

	// Update fields
	if req.SalesName != nil {
		sp.SalesName = req.SalesName
	}
	if req.SalesArea != nil {
		sp.SalesArea = req.SalesArea
	}
	if req.SalesTarget != nil {
		sp.SalesTarget = req.SalesTarget
	}
	if req.CommissionRate != nil {
		sp.CommissionRate = req.CommissionRate
	}
	if req.Whatsapp != nil {
		sp.Whatsapp = req.Whatsapp
	}
	if req.IsWhatsappConnected != nil {
		sp.IsWhatsappConnected = *req.IsWhatsappConnected
	}
	if req.IsActive != nil {
		sp.IsActive = *req.IsActive
	}
	if req.Notes != nil {
		sp.Notes = req.Notes
	}

	sp.UpdatedAt = time.Now()
	sp.UpdatedBy = &userID

	// Save to database
	if err := s.repo.Update(sp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Sales person not found during update", logger.String("id", id))
			return nil, ErrSalesPersonNotFound
		}
		logger.Error("Failed to update sales person", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully updated sales person", logger.String("id", id))

	// Fetch updated sales person
	updated, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch updated sales person", logger.Err(err))
		return nil, err
	}

	response := s.toResponse(updated)
	return &response, nil
}

// Delete soft deletes a sales person
func (s *SalesPersonService) Delete(id, companyID, userID string) error {
	logger.Info("Deleting sales person",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Check if sales person exists
	sp, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch sales person", logger.Err(err))
		return err
	}
	if sp == nil {
		logger.Warn("Sales person not found", logger.String("id", id))
		return ErrSalesPersonNotFound
	}

	// Soft delete
	if err := s.repo.Delete(id, companyID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Sales person not found during delete", logger.String("id", id))
			return ErrSalesPersonNotFound
		}
		logger.Error("Failed to delete sales person", logger.Err(err))
		return err
	}

	logger.Info("Successfully deleted sales person", logger.String("id", id))
	return nil
}

// Restore restores a soft-deleted sales person
func (s *SalesPersonService) Restore(id, companyID string) error {
	logger.Info("Restoring sales person",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Restore
	if err := s.repo.Restore(id, companyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("Sales person not found or not deleted", logger.String("id", id))
			return ErrSalesPersonNotFound
		}
		logger.Error("Failed to restore sales person", logger.Err(err))
		return err
	}

	logger.Info("Successfully restored sales person", logger.String("id", id))
	return nil
}

// toResponse converts domain entity to response DTO
func (s *SalesPersonService) toResponse(sp *domain.SalesPerson) dto.SalesPersonResponse {
	response := dto.SalesPersonResponse{
		ID:                  sp.ID,
		CompanyUserID:       sp.CompanyUserID,
		SalesCode:           sp.SalesCode,
		SalesName:           sp.SalesName,
		SalesArea:           sp.SalesArea,
		SalesTarget:         sp.SalesTarget,
		CommissionRate:      sp.CommissionRate,
		Whatsapp:            sp.Whatsapp,
		IsWhatsappConnected: sp.IsWhatsappConnected,
		IsActive:            sp.IsActive,
		Notes:               sp.Notes,
	}

	// Build user info
	if sp.UserID != nil && sp.UserFullName != nil && sp.UserEmail != nil {
		response.User = &dto.UserInfo{
			ID:       *sp.UserID,
			FullName: *sp.UserFullName,
			Email:    *sp.UserEmail,
		}
	}

	// Build audit info
	response.Audit = &dto.AuditInfo{
		CreatedAt:     sp.CreatedAt,
		CreatedBy:     sp.CreatedBy,
		CreatedByName: sp.CreatedByName,
		UpdatedAt:     sp.UpdatedAt,
		UpdatedBy:     sp.UpdatedBy,
		UpdatedByName: sp.UpdatedByName,
		DeletedAt:     sp.DeletedAt,
		DeletedBy:     sp.DeletedBy,
		DeletedByName: sp.DeletedByName,
	}

	return response
}
