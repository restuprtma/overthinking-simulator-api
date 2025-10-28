package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/dto"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/service"
	"venturo-skeleton-go/internal/shared/response"
)

type AutoReplyRuleHandler struct {
	service *service.AutoReplyRuleService
}

func NewAutoReplyRuleHandler(service *service.AutoReplyRuleService) *AutoReplyRuleHandler {
	return &AutoReplyRuleHandler{service: service}
}

// GetAll godoc
// @Summary Get all auto-reply rules
// @Description Get list of auto-reply rules with pagination and filters
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name, description, or message template"
// @Param trigger_type query string false "Filter by trigger type"
// @Param is_enabled query bool false "Filter by enabled status"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules [get]
func (h *AutoReplyRuleHandler) GetAll(c *gin.Context) {
	var params dto.AutoReplyRuleQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	rules, total, _, err := h.service.GetAll(companyID, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch auto-reply rules", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Auto-reply rules retrieved successfully", rules, params.Page, params.PageSize, int64(total))
}

// GetByID godoc
// @Summary Get auto-reply rule by ID
// @Description Get a single auto-reply rule by ID
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param id path string true "Auto-reply rule ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules/{id} [get]
func (h *AutoReplyRuleHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	rule, err := h.service.GetByID(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrAutoReplyRuleNotFound) {
			response.Error(c, http.StatusNotFound, "Auto-reply rule not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch auto-reply rule", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Auto-reply rule retrieved successfully", rule)
}

// Create godoc
// @Summary Create new auto-reply rule
// @Description Create a new auto-reply rule record
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param request body dto.CreateAutoReplyRuleRequest true "Auto-reply rule data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules [post]
func (h *AutoReplyRuleHandler) Create(c *gin.Context) {
	var req dto.CreateAutoReplyRuleRequest
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

	rule, err := h.service.Create(companyID, userID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create auto-reply rule", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Auto-reply rule created successfully", rule)
}

// Update godoc
// @Summary Update auto-reply rule
// @Description Update an existing auto-reply rule
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param id path string true "Auto-reply rule ID"
// @Param request body dto.UpdateAutoReplyRuleRequest true "Auto-reply rule data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules/{id} [put]
func (h *AutoReplyRuleHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateAutoReplyRuleRequest
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

	rule, err := h.service.Update(id, companyID, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrAutoReplyRuleNotFound) {
			response.Error(c, http.StatusNotFound, "Auto-reply rule not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update auto-reply rule", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Auto-reply rule updated successfully", rule)
}

// Delete godoc
// @Summary Delete auto-reply rule
// @Description Soft delete an auto-reply rule
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param id path string true "Auto-reply rule ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules/{id} [delete]
func (h *AutoReplyRuleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Delete(id, companyID, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete auto-reply rule", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Auto-reply rule deleted successfully", nil)
}

// Restore godoc
// @Summary Restore auto-reply rule
// @Description Restore a soft deleted auto-reply rule
// @Tags CRM - Auto Reply Rules
// @Accept json
// @Produce json
// @Param id path string true "Auto-reply rule ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/auto-reply-rules/{id}/restore [post]
func (h *AutoReplyRuleHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	err := h.service.Restore(id, companyID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to restore auto-reply rule", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Auto-reply rule restored successfully", nil)
}
