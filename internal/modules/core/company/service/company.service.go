package service

import (
	"errors"
	"math"
	"time"

	"venturo-skeleton-go/internal/modules/core/company/domain"
	"venturo-skeleton-go/internal/modules/core/company/dto"
	"venturo-skeleton-go/internal/modules/core/company/repository"
	"venturo-skeleton-go/pkg/logger"

	"github.com/google/uuid"
)

var (
	ErrCompanyNotFound       = errors.New("company not found")
	ErrCodeAlreadyExists     = errors.New("company code already exists")
	ErrUserAlreadyInCompany  = errors.New("user already in company")
	ErrUserNotInCompany      = errors.New("user not in company")
	ErrMaxUsersReached       = errors.New("maximum users limit reached for this company")
	ErrCannotRemoveOwner     = errors.New("cannot remove primary owner from company")
	ErrInvalidRole           = errors.New("invalid role")
)

type CompanyService struct {
	companyRepo     *repository.CompanyRepository
	companyUserRepo *repository.CompanyUserRepository
}

func NewCompanyService(
	companyRepo *repository.CompanyRepository,
	companyUserRepo *repository.CompanyUserRepository,
) *CompanyService {
	return &CompanyService{
		companyRepo:     companyRepo,
		companyUserRepo: companyUserRepo,
	}
}

// Create creates a new company and assigns the creator as owner
func (s *CompanyService) Create(req *dto.CreateCompanyRequest, ownerID string) (*dto.CompanyResponse, error) {
	logger.Info("Creating new company", logger.String("name", req.Name), logger.String("owner_id", ownerID))

	// Check if code already exists
	existingCompany, err := s.companyRepo.FindByCode(req.Code)
	if err != nil {
		logger.Error("Failed to check existing code", logger.Err(err))
		return nil, err
	}
	if existingCompany != nil {
		logger.Warn("Company code already exists", logger.String("code", req.Code))
		return nil, ErrCodeAlreadyExists
	}

	// Set default values if not provided
	maxUsers := 5
	if req.MaxUsers != nil {
		maxUsers = *req.MaxUsers
	}

	maxBranches := 1
	if req.MaxBranches != nil {
		maxBranches = *req.MaxBranches
	}

	// Create company
	company := &domain.Company{
		ID:          uuid.New().String(),
		OwnerID:     ownerID,
		Name:        req.Name,
		Code:        req.Code,
		TaxID:       req.TaxID,
		Phone:       req.Phone,
		Email:       req.Email,
		Website:     req.Website,
		LogoURL:     req.LogoURL,
		Address:     req.Address,
		VillageID:   req.VillageID,
		MaxUsers:    maxUsers,
		MaxBranches: maxBranches,
		IsActive:    true,
		CreatedAt:   time.Now(),
		CreatedBy:   &ownerID,
		UpdatedAt:   time.Now(),
		UpdatedBy:   &ownerID,
	}

	err = s.companyRepo.Create(company)
	if err != nil {
		logger.Error("Failed to create company", logger.Err(err))
		return nil, err
	}

	// Assign creator as primary owner
	companyUser := &domain.CompanyUser{
		ID:        uuid.New().String(),
		CompanyID: company.ID,
		UserID:    ownerID,
		RoleID:    nil, // Owner inherits global role (no company-specific role)
		Role:      domain.RoleOwner, // DEPRECATED: kept for backward compatibility
		IsPrimary: true,
		IsActive:  true,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
		CreatedBy: &ownerID,
		UpdatedAt: time.Now(),
		UpdatedBy: &ownerID,
	}

	err = s.companyUserRepo.Create(companyUser)
	if err != nil {
		logger.Error("Failed to assign owner to company", logger.Err(err))
		return nil, err
	}

	logger.Info("Company created successfully",
		logger.String("company_id", company.ID),
		logger.String("owner_id", ownerID),
	)

	return s.toCompanyResponse(company), nil
}

// GetAll gets all companies with pagination
func (s *CompanyService) GetAll(params *dto.CompanyQueryParams) (*dto.CompanyListResponse, error) {
	logger.Info("Fetching companies list",
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
		logger.String("search", params.Search),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get companies
	companies, err := s.companyRepo.FindAll(params.PageSize, offset, params.Search, params.IsActive, params.OwnerID)
	if err != nil {
		logger.Error("Failed to fetch companies", logger.Err(err))
		return nil, err
	}

	// Get total count
	total, err := s.companyRepo.Count(params.Search, params.IsActive, params.OwnerID)
	if err != nil {
		logger.Error("Failed to count companies", logger.Err(err))
		return nil, err
	}

	// Convert to response
	companyResponses := make([]dto.CompanyResponse, len(companies))
	for i, company := range companies {
		companyResponses[i] = *s.toCompanyResponse(&company)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.CompanyListResponse{
		Companies:  companyResponses,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID gets a company by ID
func (s *CompanyService) GetByID(id string) (*dto.CompanyResponse, error) {
	logger.Info("Fetching company by ID", logger.String("company_id", id))

	company, err := s.companyRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch company", logger.Err(err))
		return nil, err
	}

	if company == nil {
		logger.Warn("Company not found", logger.String("company_id", id))
		return nil, ErrCompanyNotFound
	}

	return s.toCompanyResponse(company), nil
}

// Update updates a company
func (s *CompanyService) Update(id string, req *dto.UpdateCompanyRequest, updatedBy string) (*dto.CompanyResponse, error) {
	logger.Info("Updating company", logger.String("company_id", id))

	// Get existing company
	company, err := s.companyRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch company", logger.Err(err))
		return nil, err
	}

	if company == nil {
		logger.Warn("Company not found", logger.String("company_id", id))
		return nil, ErrCompanyNotFound
	}

	// Update fields if provided
	if req.Name != nil {
		company.Name = *req.Name
	}
	if req.TaxID != nil {
		company.TaxID = req.TaxID
	}
	if req.Phone != nil {
		company.Phone = req.Phone
	}
	if req.Email != nil {
		company.Email = req.Email
	}
	if req.Website != nil {
		company.Website = req.Website
	}
	if req.LogoURL != nil {
		company.LogoURL = req.LogoURL
	}
	if req.Address != nil {
		company.Address = req.Address
	}
	if req.VillageID != nil {
		company.VillageID = req.VillageID
	}
	if req.MaxUsers != nil {
		company.MaxUsers = *req.MaxUsers
	}
	if req.MaxBranches != nil {
		company.MaxBranches = *req.MaxBranches
	}
	if req.IsActive != nil {
		company.IsActive = *req.IsActive
	}

	company.UpdatedAt = time.Now()
	company.UpdatedBy = &updatedBy

	err = s.companyRepo.Update(company)
	if err != nil {
		logger.Error("Failed to update company", logger.Err(err))
		return nil, err
	}

	logger.Info("Company updated successfully", logger.String("company_id", id))

	return s.toCompanyResponse(company), nil
}

// Delete soft deletes a company
func (s *CompanyService) Delete(id string, deletedBy string) error {
	logger.Info("Deleting company", logger.String("company_id", id))

	// Check if company exists
	company, err := s.companyRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch company", logger.Err(err))
		return err
	}

	if company == nil {
		logger.Warn("Company not found", logger.String("company_id", id))
		return ErrCompanyNotFound
	}

	err = s.companyRepo.Delete(id, deletedBy)
	if err != nil {
		logger.Error("Failed to delete company", logger.Err(err))
		return err
	}

	logger.Info("Company deleted successfully", logger.String("company_id", id))

	return nil
}

// AddUserToCompany adds a user to a company
func (s *CompanyService) AddUserToCompany(companyID string, req *dto.AddUserToCompanyRequest, invitedBy string) (*dto.CompanyUserResponse, error) {
	// Log request details for debugging
	roleIDStr := "nil"
	if req.RoleID != nil {
		roleIDStr = *req.RoleID
	}
	logger.Info("Adding user to company",
		logger.String("company_id", companyID),
		logger.String("user_id", req.UserID),
		logger.String("role_id", roleIDStr),
		logger.String("role_deprecated", req.Role),
	)

	// Validate role_id if provided
	if req.RoleID != nil && *req.RoleID != "" {
		// Validate UUID format
		if _, err := uuid.Parse(*req.RoleID); err != nil {
			logger.Error("Invalid role_id format", logger.String("role_id", *req.RoleID))
			return nil, errors.New("invalid role_id: must be a valid UUID")
		}
	}

	// Validate role if using deprecated role field
	// Priority: role_id > role (deprecated)
	if req.RoleID == nil && req.Role != "" {
		if !domain.IsValidRole(req.Role) {
			return nil, ErrInvalidRole
		}
	}

	// Check if company exists
	company, err := s.companyRepo.FindByID(companyID)
	if err != nil {
		logger.Error("Failed to fetch company", logger.Err(err))
		return nil, err
	}
	if company == nil {
		return nil, ErrCompanyNotFound
	}

	// Check if user already in company
	existing, err := s.companyUserRepo.FindByCompanyAndUser(companyID, req.UserID)
	if err != nil {
		logger.Error("Failed to check existing user", logger.Err(err))
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserAlreadyInCompany
	}

	// Check max users limit
	activeCount, err := s.companyUserRepo.CountActiveUsersByCompany(companyID)
	if err != nil {
		logger.Error("Failed to count active users", logger.Err(err))
		return nil, err
	}
	if int(activeCount) >= company.MaxUsers {
		return nil, ErrMaxUsersReached
	}

	// Determine role value for backward compatibility
	roleValue := req.Role
	if roleValue == "" {
		roleValue = domain.RoleMember // Default to MEMBER if not specified
	}

	// Add user to company
	companyUser := &domain.CompanyUser{
		ID:        uuid.New().String(),
		CompanyID: companyID,
		UserID:    req.UserID,
		RoleID:    req.RoleID, // Company-specific role ID (can be nil for global role)
		Role:      roleValue,  // DEPRECATED: kept for backward compatibility
		IsPrimary: false,
		IsActive:  true,
		InvitedBy: &invitedBy,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
		CreatedBy: &invitedBy,
		UpdatedAt: time.Now(),
		UpdatedBy: &invitedBy,
	}

	err = s.companyUserRepo.Create(companyUser)
	if err != nil {
		logger.Error("Failed to add user to company", logger.Err(err))
		return nil, err
	}

	logger.Info("User added to company successfully",
		logger.String("company_id", companyID),
		logger.String("user_id", req.UserID),
	)

	return s.toCompanyUserResponse(companyUser), nil
}

// RemoveUserFromCompany removes a user from a company
func (s *CompanyService) RemoveUserFromCompany(companyID, userID, deletedBy string) error {
	logger.Info("Removing user from company",
		logger.String("company_id", companyID),
		logger.String("user_id", userID),
	)

	// Check if user in company
	companyUser, err := s.companyUserRepo.FindByCompanyAndUser(companyID, userID)
	if err != nil {
		logger.Error("Failed to fetch company user", logger.Err(err))
		return err
	}
	if companyUser == nil {
		return ErrUserNotInCompany
	}

	// Cannot remove primary owner
	if companyUser.IsPrimary {
		return ErrCannotRemoveOwner
	}

	err = s.companyUserRepo.Delete(companyUser.ID, deletedBy)
	if err != nil {
		logger.Error("Failed to remove user from company", logger.Err(err))
		return err
	}

	logger.Info("User removed from company successfully",
		logger.String("company_id", companyID),
		logger.String("user_id", userID),
	)

	return nil
}

// UpdateUserRole updates a user's role in a company
func (s *CompanyService) UpdateUserRole(companyID, userID string, req *dto.UpdateCompanyUserRequest, updatedBy string) (*dto.CompanyUserResponse, error) {
	// Log request details for debugging
	roleIDStr := "not provided"
	if req.RoleID != nil {
		if *req.RoleID == "" {
			roleIDStr = "empty string (will set to nil)"
		} else {
			roleIDStr = *req.RoleID
		}
	}
	logger.Info("Updating user role in company",
		logger.String("company_id", companyID),
		logger.String("user_id", userID),
		logger.String("role_id", roleIDStr),
	)

	// Check if user in company
	companyUser, err := s.companyUserRepo.FindByCompanyAndUser(companyID, userID)
	if err != nil {
		logger.Error("Failed to fetch company user", logger.Err(err))
		return nil, err
	}
	if companyUser == nil {
		return nil, ErrUserNotInCompany
	}

	// Update fields if provided
	// Priority: role_id > role (deprecated)
	if req.RoleID != nil {
		// Validate UUID format if not empty
		if *req.RoleID != "" {
			if _, err := uuid.Parse(*req.RoleID); err != nil {
				logger.Error("Invalid role_id format", logger.String("role_id", *req.RoleID))
				return nil, errors.New("invalid role_id: must be a valid UUID")
			}
		} else {
			// Empty string means set to nil (inherit global role)
			req.RoleID = nil
		}
		// Update company-specific role ID (can be set to nil to use global role)
		companyUser.RoleID = req.RoleID
	}

	if req.Role != nil {
		// DEPRECATED: Only validate and update if role_id not provided
		if req.RoleID == nil {
			if !domain.IsValidRole(*req.Role) {
				return nil, ErrInvalidRole
			}
			companyUser.Role = *req.Role
		}
	}

	if req.IsActive != nil {
		companyUser.IsActive = *req.IsActive
	}

	companyUser.UpdatedAt = time.Now()
	companyUser.UpdatedBy = &updatedBy

	err = s.companyUserRepo.Update(companyUser)
	if err != nil {
		logger.Error("Failed to update user role", logger.Err(err))
		return nil, err
	}

	logger.Info("User role updated successfully",
		logger.String("company_id", companyID),
		logger.String("user_id", userID),
	)

	return s.toCompanyUserResponse(companyUser), nil
}

// GetCompanyUsers gets all users in a company
func (s *CompanyService) GetCompanyUsers(companyID string, page, pageSize int, role string, isActive *bool) (*dto.CompanyUserListResponse, error) {
	logger.Info("Fetching company users",
		logger.String("company_id", companyID),
		logger.Int("page", page),
		logger.Int("page_size", pageSize),
	)

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get users
	companyUsers, err := s.companyUserRepo.FindByCompany(companyID, pageSize, offset, role, isActive)
	if err != nil {
		logger.Error("Failed to fetch company users", logger.Err(err))
		return nil, err
	}

	// Get total count
	total, err := s.companyUserRepo.CountByCompany(companyID, role, isActive)
	if err != nil {
		logger.Error("Failed to count company users", logger.Err(err))
		return nil, err
	}

	// Convert to response
	userResponses := make([]dto.CompanyUserResponse, len(companyUsers))
	for i, cu := range companyUsers {
		userResponses[i] = *s.toCompanyUserResponse(&cu)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &dto.CompanyUserListResponse{
		Users:      userResponses,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// Helper methods

func (s *CompanyService) toCompanyResponse(company *domain.Company) *dto.CompanyResponse {
	return &dto.CompanyResponse{
		ID:          company.ID,
		OwnerID:     company.OwnerID,
		Name:        company.Name,
		Code:        company.Code,
		TaxID:       company.TaxID,
		Phone:       company.Phone,
		Email:       company.Email,
		Website:     company.Website,
		LogoURL:     company.LogoURL,
		Address:     company.Address,
		VillageID:   company.VillageID,
		MaxUsers:    company.MaxUsers,
		MaxBranches: company.MaxBranches,
		IsActive:    company.IsActive,
		CreatedAt:   company.CreatedAt,
		CreatedBy:   company.CreatedBy,
		UpdatedAt:   company.UpdatedAt,
		UpdatedBy:   company.UpdatedBy,
		DeletedAt:   company.DeletedAt,
		DeletedBy:   company.DeletedBy,
	}
}

func (s *CompanyService) toCompanyUserResponse(cu *domain.CompanyUser) *dto.CompanyUserResponse {
	createdBy := ""
	if cu.CreatedBy != nil {
		createdBy = *cu.CreatedBy
	}

	return &dto.CompanyUserResponse{
		ID:        cu.ID,
		CompanyID: cu.CompanyID,
		UserID:    cu.UserID,
		RoleID:    cu.RoleID, // Company-specific role ID (nil = inherit global role)
		Role:      cu.Role,   // DEPRECATED: kept for backward compatibility
		IsPrimary: cu.IsPrimary,
		IsActive:  cu.IsActive,
		// User information
		UserEmail:    cu.UserEmail,
		UserUsername: cu.UserUsername,
		UserFullName: cu.UserFullName,
		// Timestamps
		JoinedAt:  cu.JoinedAt,
		CreatedAt: cu.CreatedAt,
		CreatedBy: createdBy,
		UpdatedAt: cu.UpdatedAt,
		UpdatedBy: cu.UpdatedBy,
		DeletedAt: cu.DeletedAt,
		DeletedBy: cu.DeletedBy,
	}
}
