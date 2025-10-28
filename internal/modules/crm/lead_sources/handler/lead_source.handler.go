package handler

import (
	"errors"
	"net/http"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/lead_sources/dto"
	"venturo-skeleton-go/internal/modules/crm/lead_sources/service"
	"venturo-skeleton-go/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type LeadSourceHandler struct {
	service *service.LeadSourceService
}

func NewLeadSourceHandler(service *service.LeadSourceService) *LeadSourceHandler {
	return &LeadSourceHandler{
		service: service,
	}
}

// GetAll gets all lead sources with pagination
// @Summary Get all lead sources
// @Description Get list of lead sources with pagination and filters
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name, code, or description"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} response.Response "Lead sources retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources [get]
func (h *LeadSourceHandler) GetAll(c *gin.Context) {
	var params dto.LeadSourceQueryParams

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

	// Get company ID from context
	companyID := middleware.GetCompanyID(c)

	// Call service
	result, err := h.service.GetAll(companyID, &params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch lead sources", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Lead sources retrieved successfully", result.LeadSources, result.Page, result.PageSize, result.Total)
}

// GetByID gets a lead source by ID
// @Summary Get lead source by ID
// @Description Get lead source details by ID
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param id path string true "Lead Source ID"
// @Success 200 {object} response.Response "Lead source retrieved successfully"
// @Failure 404 {object} response.Response "Lead source not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources/{id} [get]
func (h *LeadSourceHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	result, err := h.service.GetByID(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrLeadSourceNotFound) {
			response.Error(c, http.StatusNotFound, "Lead source not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch lead source", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead source retrieved successfully", result)
}

// Create creates a new lead source
// @Summary Create new lead source
// @Description Create a new lead source
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param request body dto.CreateLeadSourceRequest true "Lead source data"
// @Success 201 {object} response.Response "Lead source created successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 409 {object} response.Response "Lead source code already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources [post]
func (h *LeadSourceHandler) Create(c *gin.Context) {
	var req dto.CreateLeadSourceRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	// Call service
	result, err := h.service.Create(companyID, userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrLeadSourceCodeAlreadyExists) {
			response.Error(c, http.StatusConflict, "Lead source code already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create lead source", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Lead source created successfully", result)
}

// Update updates an existing lead source
// @Summary Update lead source
// @Description Update an existing lead source
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param id path string true "Lead Source ID"
// @Param request body dto.UpdateLeadSourceRequest true "Updated lead source data"
// @Success 200 {object} response.Response "Lead source updated successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 404 {object} response.Response "Lead source not found"
// @Failure 409 {object} response.Response "Lead source code already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources/{id} [put]
func (h *LeadSourceHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req dto.UpdateLeadSourceRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	// Call service
	result, err := h.service.Update(id, companyID, userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrLeadSourceNotFound) {
			response.Error(c, http.StatusNotFound, "Lead source not found", "")
			return
		}
		if errors.Is(err, service.ErrLeadSourceCodeAlreadyExists) {
			response.Error(c, http.StatusConflict, "Lead source code already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update lead source", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead source updated successfully", result)
}

// Delete soft deletes a lead source
// @Summary Delete lead source
// @Description Soft delete a lead source
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param id path string true "Lead Source ID"
// @Success 200 {object} response.Response "Lead source deleted successfully"
// @Failure 404 {object} response.Response "Lead source not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources/{id} [delete]
func (h *LeadSourceHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	err := h.service.Delete(id, companyID, userID)
	if err != nil {
		if errors.Is(err, service.ErrLeadSourceNotFound) {
			response.Error(c, http.StatusNotFound, "Lead source not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete lead source", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead source deleted successfully", nil)
}

// Restore restores a soft-deleted lead source
// @Summary Restore lead source
// @Description Restore a soft-deleted lead source
// @Tags CRM - Lead Sources
// @Accept json
// @Produce json
// @Param id path string true "Lead Source ID"
// @Success 200 {object} response.Response "Lead source restored successfully"
// @Failure 404 {object} response.Response "Lead source not found or not deleted"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /crm/lead-sources/{id}/restore [post]
func (h *LeadSourceHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	err := h.service.Restore(id, companyID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to restore lead source", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead source restored successfully", nil)
}
