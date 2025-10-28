package handler

import (
	"errors"
	"net/http"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/permission/dto"
	"venturo-skeleton-go/internal/modules/core/permission/service"
	"venturo-skeleton-go/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permissionService *service.PermissionService
}

func NewPermissionHandler(permissionService *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
	}
}

// GetAll gets all permissions with pagination
// @Summary Get all permissions
// @Description Get list of permissions with pagination and filters
// @Tags Permissions
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by resource, action, or description"
// @Param resource query string false "Filter by resource"
// @Param action query string false "Filter by action"
// @Success 200 {object} response.Response{data=dto.PermissionListResponse} "Permissions retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /permissions [get]
func (h *PermissionHandler) GetAll(c *gin.Context) {
	var params dto.PermissionQueryParams

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
	result, err := h.permissionService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch permissions", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Permissions retrieved successfully", result.Permissions, result.Page, result.PageSize, result.Total)
}

// GetByID gets a permission by ID
// @Summary Get permission by ID
// @Description Get permission details by ID
// @Tags Permissions
// @Accept json
// @Produce json
// @Param id path string true "Permission ID"
// @Success 200 {object} response.Response{data=dto.PermissionResponse} "Permission retrieved successfully"
// @Failure 404 {object} response.Response "Permission not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /permissions/{id} [get]
func (h *PermissionHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.permissionService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrPermissionNotFound) {
			response.Error(c, http.StatusNotFound, "Permission not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch permission", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Permission retrieved successfully", result)
}

// Create creates a new permission
// @Summary Create new permission
// @Description Create a new permission (admin only)
// @Tags Permissions
// @Accept json
// @Produce json
// @Param request body dto.CreatePermissionRequest true "Permission details"
// @Success 201 {object} response.Response{data=dto.PermissionResponse} "Permission created successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 409 {object} response.Response "Permission with this resource and action already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /permissions [post]
func (h *PermissionHandler) Create(c *gin.Context) {
	var req dto.CreatePermissionRequest

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
	result, err := h.permissionService.Create(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateResourceAction) {
			response.Error(c, http.StatusConflict, "Permission with this resource and action already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create permission", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Permission created successfully", result)
}

// Update updates a permission
// @Summary Update permission
// @Description Update permission details (admin only)
// @Tags Permissions
// @Accept json
// @Produce json
// @Param id path string true "Permission ID"
// @Param request body dto.UpdatePermissionRequest true "Permission details to update"
// @Success 200 {object} response.Response{data=dto.PermissionResponse} "Permission updated successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "Permission not found"
// @Failure 409 {object} response.Response "Permission with this resource and action already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /permissions/{id} [put]
func (h *PermissionHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdatePermissionRequest
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

	result, err := h.permissionService.Update(id, &req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPermissionNotFound):
			response.Error(c, http.StatusNotFound, "Permission not found", "")
		case errors.Is(err, service.ErrDuplicateResourceAction):
			response.Error(c, http.StatusConflict, "Permission with this resource and action already exists", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update permission", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permission updated successfully", result)
}

// Delete deletes a permission
// @Summary Delete permission
// @Description Soft delete a permission (admin only)
// @Tags Permissions
// @Accept json
// @Produce json
// @Param id path string true "Permission ID"
// @Success 200 {object} response.Response "Permission deleted successfully"
// @Failure 404 {object} response.Response "Permission not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /permissions/{id} [delete]
func (h *PermissionHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	err = h.permissionService.Delete(id, userID)
	if err != nil {
		if errors.Is(err, service.ErrPermissionNotFound) {
			response.Error(c, http.StatusNotFound, "Permission not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete permission", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Permission deleted successfully", nil)
}

// GetActions gets all unique action names
// @Summary Get all unique actions
// @Description Get list of all unique action names from permissions
// @Tags Permissions
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]string}
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /permissions/actions [get]
func (h *PermissionHandler) GetActions(c *gin.Context) {
	actions, err := h.permissionService.GetActions()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve actions", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Actions retrieved successfully", actions)
}

// GetByModuleID gets all permissions for a specific module (nested resource endpoint)
// @Summary Get permissions by module ID
// @Description Get all permissions belonging to a specific module
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID (UUID)"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by resource, action, or description"
// @Param resource query string false "Filter by resource"
// @Param action query string false "Filter by action"
// @Success 200 {object} response.Response{data=dto.PermissionListResponse}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /modules/{id}/permissions [get]
func (h *PermissionHandler) GetByModuleID(c *gin.Context) {
	moduleID := c.Param("id")

	var params dto.PermissionQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	// Set module_id from URL param
	params.ModuleID = moduleID

	// Set defaults
	if params.Page == 0 {
		params.Page = 1
	}
	if params.PageSize == 0 {
		params.PageSize = 10
	}

	result, err := h.permissionService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve permissions", err.Error())
		return
	}

	response.SuccessWithPagination(
		c,
		http.StatusOK,
		"Permissions retrieved successfully",
		result.Permissions,
		result.Page,
		result.PageSize,
		result.Total,
	)
}

// ApplyCRUDTemplate creates standard CRUD permissions for a module
// @Summary Apply standard CRUD template
// @Description Automatically create create, read, update, delete permissions for a module's resource
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID (UUID)"
// @Param request body dto.ApplyCRUDTemplateRequest true "CRUD template request"
// @Success 201 {object} response.Response{data=dto.CRUDTemplateResponse}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /modules/{id}/permissions/apply-crud-template [post]
func (h *PermissionHandler) ApplyCRUDTemplate(c *gin.Context) {
	moduleID := c.Param("id")

	var req dto.ApplyCRUDTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Set module_id from URL param
	req.ModuleID = moduleID

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.permissionService.ApplyCRUDTemplate(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			response.Error(c, http.StatusNotFound, "Module not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to apply CRUD template", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "CRUD template applied successfully", result)
}
