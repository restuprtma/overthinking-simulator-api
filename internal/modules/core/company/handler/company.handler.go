package handler

import (
	"errors"
	"fmt"
	"net/http"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/company/dto"
	"venturo-skeleton-go/internal/modules/core/company/service"
	"venturo-skeleton-go/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type CompanyHandler struct {
	companyService *service.CompanyService
}

func NewCompanyHandler(companyService *service.CompanyService) *CompanyHandler {
	return &CompanyHandler{
		companyService: companyService,
	}
}

// Create creates a new company
// @Summary Create new company
// @Description Create a new company with the authenticated user as owner
// @Tags Companies
// @Accept json
// @Produce json
// @Param request body dto.CreateCompanyRequest true "Company details"
// @Success 201 {object} response.Response{data=dto.CompanyResponse} "Company created successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 409 {object} response.Response "Company code already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies [post]
func (h *CompanyHandler) Create(c *gin.Context) {
	var req dto.CreateCompanyRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	result, err := h.companyService.Create(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrCodeAlreadyExists) {
			response.Error(c, http.StatusConflict, "Company code already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create company", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Company created successfully", result)
}

// GetAll gets all companies with pagination
// @Summary Get all companies
// @Description Get list of companies with pagination and filters
// @Tags Companies
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name, code, or email"
// @Param is_active query bool false "Filter by active status"
// @Param owner_id query string false "Filter by owner ID"
// @Success 200 {object} response.Response{data=dto.CompanyListResponse} "Companies retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies [get]
func (h *CompanyHandler) GetAll(c *gin.Context) {
	var params dto.CompanyQueryParams

	// Bind query parameters
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	// Set defaults
	if params.Page == 0 {
		params.Page = 1
	}
	if params.PageSize == 0 {
		params.PageSize = 10
	}

	// Call service
	result, err := h.companyService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch companies", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Companies retrieved successfully", result.Companies, result.Page, result.PageSize, result.Total)
}

// GetByID gets a company by ID
// @Summary Get company by ID
// @Description Get company details by ID
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Success 200 {object} response.Response{data=dto.CompanyResponse} "Company retrieved successfully"
// @Failure 404 {object} response.Response "Company not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id} [get]
func (h *CompanyHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.companyService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrCompanyNotFound) {
			response.Error(c, http.StatusNotFound, "Company not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch company", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Company retrieved successfully", result)
}

// Update updates a company
// @Summary Update company
// @Description Update company details
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Param request body dto.UpdateCompanyRequest true "Company details to update"
// @Success 200 {object} response.Response{data=dto.CompanyResponse} "Company updated successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "Company not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id} [put]
func (h *CompanyHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.companyService.Update(id, &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrCompanyNotFound) {
			response.Error(c, http.StatusNotFound, "Company not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update company", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Company updated successfully", result)
}

// Delete deletes a company
// @Summary Delete company
// @Description Soft delete a company
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Success 200 {object} response.Response "Company deleted successfully"
// @Failure 404 {object} response.Response "Company not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id} [delete]
func (h *CompanyHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	err = h.companyService.Delete(id, userID)
	if err != nil {
		if errors.Is(err, service.ErrCompanyNotFound) {
			response.Error(c, http.StatusNotFound, "Company not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete company", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Company deleted successfully", nil)
}

// AddUser adds a user to a company
// @Summary Add user to company
// @Description Add a user to a company with a specific role
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Param request body dto.AddUserToCompanyRequest true "User details"
// @Success 201 {object} response.Response{data=dto.CompanyUserResponse} "User added to company successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "Company not found"
// @Failure 409 {object} response.Response "User already in company or max users reached"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id}/users [post]
func (h *CompanyHandler) AddUser(c *gin.Context) {
	companyID := c.Param("id")

	var req dto.AddUserToCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get user ID from context (inviter)
	inviterID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.companyService.AddUserToCompany(companyID, &req, inviterID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCompanyNotFound):
			response.Error(c, http.StatusNotFound, "Company not found", "")
		case errors.Is(err, service.ErrUserAlreadyInCompany):
			response.Error(c, http.StatusConflict, "User already in company", "")
		case errors.Is(err, service.ErrMaxUsersReached):
			response.Error(c, http.StatusConflict, "Maximum users limit reached", "")
		case errors.Is(err, service.ErrInvalidRole):
			response.Error(c, http.StatusBadRequest, "Invalid role", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to add user to company", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "User added to company successfully", result)
}

// RemoveUser removes a user from a company
// @Summary Remove user from company
// @Description Remove a user from a company
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Param user_id path string true "User ID"
// @Success 200 {object} response.Response "User removed from company successfully"
// @Failure 400 {object} response.Response "Cannot remove primary owner"
// @Failure 404 {object} response.Response "User not in company"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id}/users/{user_id} [delete]
func (h *CompanyHandler) RemoveUser(c *gin.Context) {
	companyID := c.Param("id")
	userID := c.Param("user_id")

	// Get user ID from context (who is removing)
	deletedBy, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	err = h.companyService.RemoveUserFromCompany(companyID, userID, deletedBy)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotInCompany):
			response.Error(c, http.StatusNotFound, "User not in company", "")
		case errors.Is(err, service.ErrCannotRemoveOwner):
			response.Error(c, http.StatusBadRequest, "Cannot remove primary owner from company", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to remove user from company", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "User removed from company successfully", nil)
}

// UpdateUserRole updates a user's role in a company
// @Summary Update user role in company
// @Description Update a user's role or status in a company
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Param user_id path string true "User ID"
// @Param request body dto.UpdateCompanyUserRequest true "Update details"
// @Success 200 {object} response.Response{data=dto.CompanyUserResponse} "User role updated successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "User not in company"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id}/users/{user_id} [put]
func (h *CompanyHandler) UpdateUserRole(c *gin.Context) {
	companyID := c.Param("id")
	userID := c.Param("user_id")

	var req dto.UpdateCompanyUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get user ID from context
	updatedBy, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.companyService.UpdateUserRole(companyID, userID, &req, updatedBy)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotInCompany):
			response.Error(c, http.StatusNotFound, "User not in company", "")
		case errors.Is(err, service.ErrInvalidRole):
			response.Error(c, http.StatusBadRequest, "Invalid role", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update user role", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "User role updated successfully", result)
}

// GetUsers gets all users in a company
// @Summary Get company users
// @Description Get list of users in a company
// @Tags Companies
// @Accept json
// @Produce json
// @Param id path string true "Company ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param role query string false "Filter by role"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} response.Response{data=dto.CompanyUserListResponse} "Company users retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /companies/{id}/users [get]
func (h *CompanyHandler) GetUsers(c *gin.Context) {
	companyID := c.Param("id")

	// Get query params
	page := 1
	pageSize := 10
	role := c.Query("role")
	var isActive *bool

	if c.Query("page") != "" {
		if _, err := fmt.Sscanf(c.Query("page"), "%d", &page); err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid page parameter", "")
			return
		}
	}

	if c.Query("page_size") != "" {
		if _, err := fmt.Sscanf(c.Query("page_size"), "%d", &pageSize); err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid page_size parameter", "")
			return
		}
	}

	if c.Query("is_active") != "" {
		val := c.Query("is_active") == "true"
		isActive = &val
	}

	result, err := h.companyService.GetCompanyUsers(companyID, page, pageSize, role, isActive)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch company users", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Company users retrieved successfully", result.Users, result.Page, result.PageSize, result.Total)
}
