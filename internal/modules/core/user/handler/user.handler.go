package handler

import (
	"errors"
	"net/http"

	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/user/dto"
	"lakukan-be/internal/modules/core/user/service"
	"lakukan-be/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetAll gets all users with pagination
// @Summary Get all users
// @Description Get list of users with pagination and filters
// @Tags Users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search across email, username, and full name (OR condition)"
// @Param full_name query string false "Filter by full name (partial match)"
// @Param username query string false "Filter by username (partial match)"
// @Param email query string false "Filter by email (partial match)"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} response.Response{data=dto.UserListResponse} "Users retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users [get]
func (h *UserHandler) GetAll(c *gin.Context) {
	var params dto.UserQueryParams

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
	result, err := h.userService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch users", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Users retrieved successfully", result.Users, result.Page, result.PageSize, result.Total)
}

// GetByID gets a user by ID
// @Summary Get user by ID
// @Description Get user details by ID
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} response.Response{data=dto.UserResponse} "User retrieved successfully"
// @Failure 404 {object} response.Response "User not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.userService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", result)
}

// Create creates a new user
// @Summary Create new user
// @Description Create a new user account
// @Tags Users
// @Accept json
// @Produce json
// @Param request body dto.CreateUserRequest true "User details"
// @Success 201 {object} response.Response{data=dto.UserResponse} "User created successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 409 {object} response.Response "Email or username already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get current user ID from JWT context
	userID := middleware.MustGetUserID(c)

	// Call service
	result, err := h.userService.Create(userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "Email already exists", "")
		case errors.Is(err, service.ErrUsernameAlreadyExists):
			response.Error(c, http.StatusConflict, "Username already exists", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create user", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "User created successfully", result)
}

// Update updates a user
// @Summary Update user
// @Description Update user details
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.UpdateUserRequest true "User details to update"
// @Success 200 {object} response.Response{data=dto.UserResponse} "User updated successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "User not found"
// @Failure 409 {object} response.Response "Email or username already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateUserRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get current user ID from JWT context
	userID := middleware.MustGetUserID(c)

	// Call service
	result, err := h.userService.Update(id, userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "Email already exists", "")
		case errors.Is(err, service.ErrUsernameAlreadyExists):
			response.Error(c, http.StatusConflict, "Username already exists", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update user", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "User updated successfully", result)
}

// Delete deletes a user
// @Summary Delete user
// @Description Soft delete a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} response.Response "User deleted successfully"
// @Failure 404 {object} response.Response "User not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	err = h.userService.Delete(id, claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User deleted successfully", nil)
}

// Restore restores a soft-deleted user
// @Summary Restore deleted user
// @Description Restore a soft-deleted user by ID
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} response.Response{data=dto.UserResponse} "User restored successfully"
// @Failure 404 {object} response.Response "User not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/restore [post]
func (h *UserHandler) Restore(c *gin.Context) {
	id := c.Param("id")

	user, err := h.userService.Restore(id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found or already active", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to restore user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User restored successfully", user)
}

// ChangePassword changes user password
// @Summary Change password
// @Description Change user password (users can only change their own password unless they have users:update permission)
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.ChangePasswordRequest true "Password change request"
// @Success 200 {object} response.Response "Password changed successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 401 {object} response.Response "Invalid old password"
// @Failure 403 {object} response.Response "Forbidden - Cannot change other user's password"
// @Failure 404 {object} response.Response "User not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/change-password [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	id := c.Param("id")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	// Check authorization: user can only change their own password
	// Unless they have users:update permission (admin/super_admin)
	if claims.UserID != id && !claims.HasPermission("users:update") {
		response.Error(c, http.StatusForbidden, "Forbidden", "You can only change your own password")
		return
	}

	var req dto.ChangePasswordRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	err = h.userService.ChangePassword(id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrInvalidPassword):
			response.Error(c, http.StatusUnauthorized, "Invalid old password", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to change password", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}

// AssignRoles assigns roles to a user
// @Summary Assign roles to user
// @Description Assign one or more roles to a user (adds to existing roles)
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.AssignRolesRequest true "Role IDs to assign"
// @Success 200 {object} response.Response{data=dto.UserResponse} "Roles assigned successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "User not found or invalid role ID"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/roles [post]
func (h *UserHandler) AssignRoles(c *gin.Context) {
	userID := c.Param("id")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	var req dto.AssignRolesRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.userService.AssignRolesToUser(userID, &req, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrInvalidRoleID):
			response.Error(c, http.StatusBadRequest, "Invalid role ID", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to assign roles", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Roles assigned successfully", result)
}

// RemoveRole removes a role from a user
// @Summary Remove role from user
// @Description Remove a specific role from a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param roleId path string true "Role ID"
// @Success 200 {object} response.Response{data=dto.UserResponse} "Role removed successfully"
// @Failure 404 {object} response.Response "User not found or role not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/roles/{roleId} [delete]
func (h *UserHandler) RemoveRole(c *gin.Context) {
	userID := c.Param("id")
	roleID := c.Param("roleId")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	// Call service
	result, err := h.userService.RemoveRoleFromUser(userID, roleID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrRoleNotFound):
			response.Error(c, http.StatusNotFound, "Role not found", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to remove role", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Role removed successfully", result)
}

// SyncRoles syncs/replaces all user roles
// @Summary Sync user roles
// @Description Replace all user roles with a new set (removes old roles, assigns new ones)
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.SyncRolesRequest true "Role IDs to sync"
// @Success 200 {object} response.Response{data=dto.UserResponse} "Roles synced successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 404 {object} response.Response "User not found or invalid role ID"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/roles [put]
func (h *UserHandler) SyncRoles(c *gin.Context) {
	userID := c.Param("id")

	// Get current user from JWT context
	claims, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", "User not authenticated")
		return
	}

	var req dto.SyncRolesRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.userService.SyncUserRoles(userID, &req, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrInvalidRoleID):
			response.Error(c, http.StatusBadRequest, "Invalid role ID", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to sync roles", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Roles synced successfully", result)
}