package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/leads/dto"
	"lakukan-be/internal/modules/crm/leads/service"
	"lakukan-be/internal/shared/response"
)

type LeadHandler struct {
	service *service.LeadService
}

func NewLeadHandler(service *service.LeadService) *LeadHandler {
	return &LeadHandler{service: service}
}

// GetAll godoc
// @Summary Get all leads
// @Description Get list of leads with pagination and filters
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name, phone, email, or contact person"
// @Param category query string false "Filter by category (hot, warm, cold)"
// @Param status query string false "Filter by status (new, in_progress, follow_up, negotiation, won, lost)"
// @Param source_id query string false "Filter by source ID"
// @Param assigned_to_company_user_id query string false "Filter by assigned user"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads [get]
func (h *LeadHandler) GetAll(c *gin.Context) {
	var params dto.LeadQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	leads, total, _, err := h.service.GetAll(companyID, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch leads", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Leads retrieved successfully", leads, params.Page, params.PageSize, int64(total))
}

// GetByID godoc
// @Summary Get lead by ID
// @Description Get a single lead by ID
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param id path string true "Lead ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads/{id} [get]
func (h *LeadHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	lead, err := h.service.GetByID(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrLeadNotFound) {
			response.Error(c, http.StatusNotFound, "Lead not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch lead", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead retrieved successfully", lead)
}

// Create godoc
// @Summary Create new lead
// @Description Create a new lead record
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param request body dto.CreateLeadRequest true "Lead data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads [post]
func (h *LeadHandler) Create(c *gin.Context) {
	var req dto.CreateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	lead, err := h.service.Create(companyID, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrPhoneAlreadyExists) {
			response.Error(c, http.StatusConflict, "Phone number already exists", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create lead", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Lead created successfully", lead)
}

// Update godoc
// @Summary Update lead
// @Description Update an existing lead
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param id path string true "Lead ID"
// @Param request body dto.UpdateLeadRequest true "Lead data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads/{id} [put]
func (h *LeadHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	lead, err := h.service.Update(id, companyID, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrLeadNotFound) {
			response.Error(c, http.StatusNotFound, "Lead not found", "")
			return
		}
		if errors.Is(err, service.ErrPhoneAlreadyExists) {
			response.Error(c, http.StatusConflict, "Phone number already exists", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update lead", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead updated successfully", lead)
}

// Delete godoc
// @Summary Delete lead
// @Description Soft delete a lead
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param id path string true "Lead ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads/{id} [delete]
func (h *LeadHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Delete(id, companyID, userID)
	if err != nil {
		if errors.Is(err, service.ErrLeadNotFound) {
			response.Error(c, http.StatusNotFound, "Lead not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete lead", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead deleted successfully", nil)
}

// Restore godoc
// @Summary Restore deleted lead
// @Description Restore a soft-deleted lead
// @Tags CRM - Leads
// @Accept json
// @Produce json
// @Param id path string true "Lead ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/leads/{id}/restore [post]
func (h *LeadHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Restore(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrLeadNotFound) {
			response.Error(c, http.StatusNotFound, "Lead not found or not deleted", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to restore lead", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Lead restored successfully", nil)
}
