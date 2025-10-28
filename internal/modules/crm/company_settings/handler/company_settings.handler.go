package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/company_settings/dto"
	"lakukan-be/internal/modules/crm/company_settings/service"
	"lakukan-be/internal/shared/response"
)

type CompanySettingsHandler struct {
	service *service.CompanySettingsService
}

func NewCompanySettingsHandler(service *service.CompanySettingsService) *CompanySettingsHandler {
	return &CompanySettingsHandler{service: service}
}

// Get godoc
// @Summary Get company CRM settings
// @Description Get CRM settings for the current company (creates default if not exists)
// @Tags CRM - Company Settings
// @Accept json
// @Produce json
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/settings [get]
func (h *CompanySettingsHandler) Get(c *gin.Context) {
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	settings, err := h.service.Get(companyID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch company settings", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Company settings retrieved successfully", settings)
}

// Update godoc
// @Summary Update company CRM settings
// @Description Update CRM settings for the current company
// @Tags CRM - Company Settings
// @Accept json
// @Produce json
// @Param request body dto.UpdateCompanySettingsRequest true "Settings data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/settings [put]
func (h *CompanySettingsHandler) Update(c *gin.Context) {
	var req dto.UpdateCompanySettingsRequest
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

	settings, err := h.service.Update(companyID, userID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update company settings", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Company settings updated successfully", settings)
}
