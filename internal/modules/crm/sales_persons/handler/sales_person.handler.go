package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/sales_persons/dto"
	"lakukan-be/internal/modules/crm/sales_persons/service"
	"lakukan-be/internal/shared/response"
)

type SalesPersonHandler struct {
	service        *service.SalesPersonService
	whatsappService *service.WhatsAppSessionService
}

func NewSalesPersonHandler(service *service.SalesPersonService, whatsappService *service.WhatsAppSessionService) *SalesPersonHandler {
	return &SalesPersonHandler{
		service:        service,
		whatsappService: whatsappService,
	}
}

// GetAll godoc
// @Summary Get all sales persons
// @Description Get list of sales persons with pagination and filters
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by sales code, name, user name, or area"
// @Param is_active query boolean false "Filter by active status"
// @Param is_whatsapp_connected query boolean false "Filter by WhatsApp connection status"
// @Param sales_area query string false "Filter by sales area"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons [get]
func (h *SalesPersonHandler) GetAll(c *gin.Context) {
	var params dto.SalesPersonQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	salesPersons, total, _, err := h.service.GetAll(companyID, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch sales persons", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Sales persons retrieved successfully", salesPersons, params.Page, params.PageSize, int64(total))
}

// GetByID godoc
// @Summary Get sales person by ID
// @Description Get a single sales person by ID
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id} [get]
func (h *SalesPersonHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	salesPerson, err := h.service.GetByID(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch sales person", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Sales person retrieved successfully", salesPerson)
}

// Create godoc
// @Summary Create new sales person
// @Description Create a new sales person record
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param request body dto.CreateSalesPersonRequest true "Sales person data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons [post]
func (h *SalesPersonHandler) Create(c *gin.Context) {
	var req dto.CreateSalesPersonRequest
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

	salesPerson, err := h.service.Create(companyID, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrSalesCodeAlreadyExists) {
			response.Error(c, http.StatusConflict, "Sales code already exists", err.Error())
			return
		}
		if errors.Is(err, service.ErrCompanyUserAlreadyHasSalesPerson) {
			response.Error(c, http.StatusConflict, "This company user already has a sales person record", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create sales person", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Sales person created successfully", salesPerson)
}

// Update godoc
// @Summary Update sales person
// @Description Update an existing sales person
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Param request body dto.UpdateSalesPersonRequest true "Sales person data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id} [put]
func (h *SalesPersonHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateSalesPersonRequest
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

	salesPerson, err := h.service.Update(id, companyID, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		if errors.Is(err, service.ErrSalesCodeAlreadyExists) {
			response.Error(c, http.StatusConflict, "Sales code already exists", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update sales person", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Sales person updated successfully", salesPerson)
}

// Delete godoc
// @Summary Delete sales person
// @Description Soft delete a sales person
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id} [delete]
func (h *SalesPersonHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Delete(id, companyID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete sales person", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Sales person deleted successfully", nil)
}

// Restore godoc
// @Summary Restore deleted sales person
// @Description Restore a soft-deleted sales person
// @Tags CRM - Sales Persons
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id}/restore [post]
func (h *SalesPersonHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Restore(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found or not deleted", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to restore sales person", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Sales person restored successfully", nil)
}

// ConnectWhatsApp godoc
// @Summary Connect WhatsApp for sales person
// @Description Create WAHA session and return pairing code for WhatsApp connection
// @Tags CRM - Sales Persons - WhatsApp
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 201 {object} response.Response{data=dto.ConnectWhatsAppResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id}/whatsapp/connect [post]
func (h *SalesPersonHandler) ConnectWhatsApp(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	result, err := h.whatsappService.ConnectWhatsApp(id, companyID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		if errors.Is(err, service.ErrWhatsappNumberRequired) {
			response.Error(c, http.StatusBadRequest, "WhatsApp number is required for this sales person", err.Error())
			return
		}
		if errors.Is(err, service.ErrWhatsappAlreadyConnected) {
			response.Error(c, http.StatusConflict, "WhatsApp is already connected", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to connect WhatsApp", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "WhatsApp session created. Please enter the pairing code in your WhatsApp app.", result)
}

// GetWhatsAppStatus godoc
// @Summary Get WhatsApp connection status
// @Description Get current WhatsApp connection status for sales person
// @Tags CRM - Sales Persons - WhatsApp
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 200 {object} response.Response{data=dto.WhatsAppStatusResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id}/whatsapp/status [get]
func (h *SalesPersonHandler) GetWhatsAppStatus(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	status, err := h.whatsappService.GetWhatsAppStatus(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get WhatsApp status", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "WhatsApp status retrieved successfully", status)
}

// DisconnectWhatsApp godoc
// @Summary Disconnect WhatsApp
// @Description Disconnect and delete WhatsApp session for sales person
// @Tags CRM - Sales Persons - WhatsApp
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Param body body dto.DisconnectWhatsAppRequest false "Disconnect options"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id}/whatsapp/disconnect [post]
func (h *SalesPersonHandler) DisconnectWhatsApp(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	var req dto.DisconnectWhatsAppRequest
	// Body is optional, default to logout=true
	logout := true
	if err := c.ShouldBindJSON(&req); err == nil {
		logout = req.Logout
	}

	err := h.whatsappService.DisconnectWhatsApp(id, companyID, userID, logout)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		if errors.Is(err, service.ErrWhatsappNotConnected) {
			response.Error(c, http.StatusBadRequest, "WhatsApp is not connected", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to disconnect WhatsApp", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "WhatsApp disconnected successfully", nil)
}

// RestartWhatsApp godoc
// @Summary Restart WhatsApp session
// @Description Restart WhatsApp session for sales person
// @Tags CRM - Sales Persons - WhatsApp
// @Accept json
// @Produce json
// @Param id path string true "Sales Person ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/sales-persons/{id}/whatsapp/restart [post]
func (h *SalesPersonHandler) RestartWhatsApp(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.whatsappService.RestartWhatsApp(id, companyID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSalesPersonNotFound) {
			response.Error(c, http.StatusNotFound, "Sales person not found", "")
			return
		}
		if errors.Is(err, service.ErrSessionNotFound) {
			response.Error(c, http.StatusBadRequest, "No WhatsApp session found to restart", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to restart WhatsApp session", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "WhatsApp session restarted successfully", nil)
}
