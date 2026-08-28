package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/user/dto"
	"venturo-skeleton-go/internal/modules/core/user/service"
	"venturo-skeleton-go/internal/shared/response"
)

type UserHandler struct {
	userService     *service.UserService
	companyVerifier middleware.CompanyContextVerifier
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// SetCompanyVerifier wires the company verifier (optional)
func (h *UserHandler) SetCompanyVerifier(v middleware.CompanyContextVerifier) {
	h.companyVerifier = v
}

func (h *UserHandler) resolveTenantScope(c *gin.Context) (*service.TenantScope, error) {
	return nil, nil
}

func (h *UserHandler) GetAll(c *gin.Context) {
	var params dto.UserQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	scope, _ := h.resolveTenantScope(c)

	ctx := c.Request.Context()
	result, err := h.userService.GetAll(ctx, &params, scope)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get users", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Users retrieved successfully",
		result.Users, result.Page, result.Limit, result.Total)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "User ID is required", "")
		return
	}

	scope, _ := h.resolveTenantScope(c)

	ctx := c.Request.Context()
	result, err := h.userService.GetByID(ctx, id, scope)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", result)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	scope, _ := h.resolveTenantScope(c)

	createdBy := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	result, err := h.userService.Create(ctx, &req, createdBy, scope)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "Email already exists", "")
		case errors.Is(err, service.ErrUsernameAlreadyExists):
			response.Error(c, http.StatusConflict, "Username already exists", "")
		case errors.Is(err, service.ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, "Role not allowed for caller", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create user", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "User created successfully", result)
}

func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "User ID is required", "")
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	scope, _ := h.resolveTenantScope(c)

	updatedBy := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	result, err := h.userService.Update(ctx, id, &req, updatedBy, scope)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "Email already exists", "")
		case errors.Is(err, service.ErrUsernameAlreadyExists):
			response.Error(c, http.StatusConflict, "Username already exists", "")
		case errors.Is(err, service.ErrRoleNotAllowed):
			response.Error(c, http.StatusForbidden, "Role not allowed for caller", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to update user", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "User updated successfully", result)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "User ID is required", "")
		return
	}

	scope, _ := h.resolveTenantScope(c)

	deletedBy := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	err := h.userService.Delete(ctx, id, deletedBy, scope)
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

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	result, err := h.userService.GetMe(ctx, userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", result)
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var req dto.UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	result, err := h.userService.UpdateMe(ctx, userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "User not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update user", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User updated successfully", result)
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID := middleware.MustGetUserID(c)
	ctx := c.Request.Context()

	err := h.userService.ChangePassword(ctx, userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrInvalidPassword):
			response.Error(c, http.StatusBadRequest, "Current password is incorrect", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to change password", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}
