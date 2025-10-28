package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/permission_template/dto"
	"lakukan-be/internal/modules/core/permission_template/service"
	"lakukan-be/internal/shared/response"
)

type PermissionTemplateHandler struct {
	templateService *service.PermissionTemplateService
}

func NewPermissionTemplateHandler(templateService *service.PermissionTemplateService) *PermissionTemplateHandler {
	return &PermissionTemplateHandler{
		templateService: templateService,
	}
}

// GetAll gets all permission templates with pagination
// @Summary Get all permission templates
// @Description Get list of permission templates with pagination and filters
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name or description"
// @Param include_system query bool false "Include system templates"
// @Success 200 {object} response.Response{data=dto.PermissionTemplateListResponse}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates [get]
func (h *PermissionTemplateHandler) GetAll(c *gin.Context) {
	var params dto.PermissionTemplateQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	result, err := h.templateService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve permission templates", err.Error())
		return
	}

	response.SuccessWithPagination(
		c,
		http.StatusOK,
		"Permission templates retrieved successfully",
		result.Templates,
		result.Page,
		result.PageSize,
		result.Total,
	)
}

// GetByID gets a permission template by ID
// @Summary Get permission template by ID
// @Description Get a single permission template with its actions
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param id path string true "Permission Template ID"
// @Success 200 {object} response.Response{data=dto.PermissionTemplateResponse}
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates/{id} [get]
func (h *PermissionTemplateHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.templateService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrTemplateNotFound) {
			response.Error(c, http.StatusNotFound, "Permission template not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve permission template", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Permission template retrieved successfully", result)
}

// Create creates a new permission template
// @Summary Create a new permission template
// @Description Create a new permission template with actions
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param request body dto.CreatePermissionTemplateRequest true "Create permission template request"
// @Success 201 {object} response.Response{data=dto.PermissionTemplateResponse}
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates [post]
func (h *PermissionTemplateHandler) Create(c *gin.Context) {
	var req dto.CreatePermissionTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.templateService.Create(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrTemplateNameExists) {
			response.Error(c, http.StatusConflict, "Permission template name already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create permission template", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Permission template created successfully", result)
}

// Update updates an existing permission template
// @Summary Update a permission template
// @Description Update an existing permission template
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param id path string true "Permission Template ID"
// @Param request body dto.UpdatePermissionTemplateRequest true "Update permission template request"
// @Success 200 {object} response.Response{data=dto.PermissionTemplateResponse}
// @Failure 400 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates/{id} [put]
func (h *PermissionTemplateHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdatePermissionTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.templateService.Update(id, &req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTemplateNotFound):
			response.Error(c, http.StatusNotFound, "Permission template not found", "")
		case errors.Is(err, service.ErrSystemTemplateCannotModify):
			response.Error(c, http.StatusForbidden, "System templates cannot be modified", "")
		case errors.Is(err, service.ErrTemplateNameExists):
			response.Error(c, http.StatusConflict, "Permission template name already exists", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update permission template", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permission template updated successfully", result)
}

// Delete soft deletes a permission template
// @Summary Delete a permission template
// @Description Soft delete a permission template
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param id path string true "Permission Template ID"
// @Success 200 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates/{id} [delete]
func (h *PermissionTemplateHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	err = h.templateService.Delete(id, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTemplateNotFound):
			response.Error(c, http.StatusNotFound, "Permission template not found", "")
		case errors.Is(err, service.ErrSystemTemplateCannotModify):
			response.Error(c, http.StatusForbidden, "System templates cannot be deleted", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to delete permission template", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permission template deleted successfully", nil)
}

// Restore restores a soft deleted permission template
// @Summary Restore a permission template
// @Description Restore a soft deleted permission template
// @Tags Permission Templates
// @Accept json
// @Produce json
// @Param id path string true "Permission Template ID"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permission-templates/{id}/restore [post]
func (h *PermissionTemplateHandler) Restore(c *gin.Context) {
	id := c.Param("id")

	err := h.templateService.Restore(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to restore permission template", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Permission template restored successfully", nil)
}
