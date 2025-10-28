package handler

import (
	"errors"
	"net/http"

	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/role/dto"
	"lakukan-be/internal/modules/core/role/service"
	"lakukan-be/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type RoleHandler struct {
	roleService *service.RoleService
}

func NewRoleHandler(roleService *service.RoleService) *RoleHandler {
	return &RoleHandler{
		roleService: roleService,
	}
}

// GetAll gets all roles with pagination
// @Summary Get all roles
// @Description Get list of roles with pagination and filters
// @Tags Roles
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name or description"
// @Param include_system query bool false "Include system roles" default(true)
// @Success 200 {object} response.Response{data=dto.RoleListResponse} "Roles retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles [get]
func (h *RoleHandler) GetAll(c *gin.Context) {
	var params dto.RoleQueryParams

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
	result, err := h.roleService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch roles", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Roles retrieved successfully", result.Roles, result.Page, result.PageSize, result.Total)
}

// GetByID gets a role by ID
// @Summary Get role by ID
// @Description Get role details by ID
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} response.Response{data=dto.RoleResponse} "Role retrieved successfully"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [get]
func (h *RoleHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.roleService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			response.Error(c, http.StatusNotFound, "Role not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch role", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Role retrieved successfully", result)
}

// Create creates a new role
// @Summary Create new role
// @Description Create a new role
// @Tags Roles
// @Accept json
// @Produce json
// @Param request body dto.CreateRoleRequest true "Role details"
// @Success 201 {object} response.Response{data=dto.RoleResponse} "Role created successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 409 {object} response.Response "Role name already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles [post]
func (h *RoleHandler) Create(c *gin.Context) {
	var req dto.CreateRoleRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get current user ID
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	result, err := h.roleService.Create(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrRoleNameAlreadyExists) {
			response.Error(c, http.StatusConflict, "Role name already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create role", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Role created successfully", result)
}

// Update updates a role
// @Summary Update role
// @Description Update role details
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body dto.UpdateRoleRequest true "Role details to update"
// @Success 200 {object} response.Response{data=dto.RoleResponse} "Role updated successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 403 {object} response.Response "Cannot modify system role"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 409 {object} response.Response "Role name already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [put]
func (h *RoleHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateRoleRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get current user ID
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	result, err := h.roleService.Update(id, &req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		case errors.Is(err, service.ErrSystemRoleCannotModify):
			response.Error(c, http.StatusForbidden, "Cannot modify system role", "")
		case errors.Is(err, service.ErrRoleNameAlreadyExists):
			response.Error(c, http.StatusConflict, "Role name already exists", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update role", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Role updated successfully", result)
}

// Delete deletes a role
// @Summary Delete role
// @Description Soft delete a role (system roles cannot be deleted)
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} response.Response "Role deleted successfully"
// @Failure 403 {object} response.Response "Cannot delete system role"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	err = h.roleService.Delete(id, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		case errors.Is(err, service.ErrSystemRoleCannotModify):
			response.Error(c, http.StatusForbidden, "Cannot delete system role", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to delete role", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Role deleted successfully", nil)
}

// Restore restores a soft-deleted role
// @Summary Restore deleted role
// @Description Restore a soft-deleted role by ID
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} response.Response{data=dto.RoleResponse} "Role restored successfully"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id}/restore [post]
func (h *RoleHandler) Restore(c *gin.Context) {
	id := c.Param("id")

	role, err := h.roleService.Restore(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			response.Error(c, http.StatusNotFound, "Role not found or already active", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to restore role", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Role restored successfully", role)
}

// GetRolePermissions gets all permissions for a specific role
// @Summary Get role permissions
// @Description Get all permissions assigned to a specific role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} response.Response{data=dto.RoleWithPermissionsResponse} "Role permissions retrieved successfully"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id}/permissions [get]
func (h *RoleHandler) GetRolePermissions(c *gin.Context) {
	id := c.Param("id")

	result, err := h.roleService.GetRoleWithPermissions(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			response.Error(c, http.StatusNotFound, "Role not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch role permissions", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Role permissions retrieved successfully", result)
}

// AssignPermissions assigns permissions to a role (replaces all existing permissions)
// @Summary Assign permissions to role
// @Description Assign permissions to a role (replaces all existing permissions)
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param request body dto.AssignPermissionsRequest true "Permission IDs to assign"
// @Success 200 {object} response.Response "Permissions assigned successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 403 {object} response.Response "Cannot modify system role"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id}/permissions [put]
func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id := c.Param("id")

	var req dto.AssignPermissionsRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	err := h.roleService.AssignPermissions(id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		case errors.Is(err, service.ErrSystemRoleCannotModify):
			response.Error(c, http.StatusForbidden, "Cannot modify system role permissions", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to assign permissions", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permissions assigned successfully", nil)
}

// AddPermission adds a single permission to a role
// @Summary Add permission to role
// @Description Add a single permission to a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param permission_id path string true "Permission ID"
// @Success 200 {object} response.Response "Permission added successfully"
// @Failure 403 {object} response.Response "Cannot modify system role"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id}/permissions/{permission_id} [post]
func (h *RoleHandler) AddPermission(c *gin.Context) {
	id := c.Param("id")
	permissionID := c.Param("permission_id")

	err := h.roleService.AddPermission(id, permissionID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		case errors.Is(err, service.ErrSystemRoleCannotModify):
			response.Error(c, http.StatusForbidden, "Cannot modify system role permissions", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to add permission", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permission added successfully", nil)
}

// RemovePermission removes a single permission from a role
// @Summary Remove permission from role
// @Description Remove a single permission from a role
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param permission_id path string true "Permission ID"
// @Success 200 {object} response.Response "Permission removed successfully"
// @Failure 403 {object} response.Response "Cannot modify system role"
// @Failure 404 {object} response.Response "Role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /roles/{id}/permissions/{permission_id} [delete]
func (h *RoleHandler) RemovePermission(c *gin.Context) {
	id := c.Param("id")
	permissionID := c.Param("permission_id")

	err := h.roleService.RemovePermission(id, permissionID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		case errors.Is(err, service.ErrSystemRoleCannotModify):
			response.Error(c, http.StatusForbidden, "Cannot modify system role permissions", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to remove permission", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Permission removed successfully", nil)
}
