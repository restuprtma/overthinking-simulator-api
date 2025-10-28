package handler

import (
	"errors"
	"net/http"

	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/permission/dto"
	"lakukan-be/internal/modules/core/permission/service"
	"lakukan-be/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type ModuleHandler struct {
	moduleService *service.ModuleService
}

func NewModuleHandler(moduleService *service.ModuleService) *ModuleHandler {
	return &ModuleHandler{
		moduleService: moduleService,
	}
}

// GetAll gets all modules with pagination
// @Summary Get all modules
// @Description Get list of modules with pagination and filters
// @Tags Modules
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by code, name, or description"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} response.Response{data=dto.ModuleListResponse} "Modules retrieved successfully"
// @Failure 400 {object} response.Response "Invalid query parameters"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules [get]
func (h *ModuleHandler) GetAll(c *gin.Context) {
	var params dto.ModuleQueryParams

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
	result, err := h.moduleService.GetAll(&params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch modules", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Modules retrieved successfully", result.Modules, result.Page, result.PageSize, result.Total)
}

// GetByID gets a module by ID
// @Summary Get module by ID
// @Description Get module details by ID
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID"
// @Success 200 {object} response.Response{data=dto.ModuleDetailResponse} "Module retrieved successfully"
// @Failure 404 {object} response.Response "Module not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules/{id} [get]
func (h *ModuleHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.moduleService.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			response.Error(c, http.StatusNotFound, "Module not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch module", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Module retrieved successfully", result)
}

// Create creates a new module
// @Summary Create new module
// @Description Create a new module (admin only)
// @Tags Modules
// @Accept json
// @Produce json
// @Param module body dto.CreateModuleRequest true "Module data"
// @Success 201 {object} response.Response{data=dto.ModuleDetailResponse} "Module created successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 409 {object} response.Response "Module code already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules [post]
func (h *ModuleHandler) Create(c *gin.Context) {
	var req dto.CreateModuleRequest

	// Bind request body
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	result, err := h.moduleService.Create(&req, userID)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateModuleCode) {
			response.Error(c, http.StatusConflict, "Module code already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create module", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Module created successfully", result)
}

// Update updates a module
// @Summary Update module
// @Description Update module details (admin only)
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID"
// @Param module body dto.UpdateModuleRequest true "Module data"
// @Success 200 {object} response.Response{data=dto.ModuleDetailResponse} "Module updated successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 404 {object} response.Response "Module not found"
// @Failure 409 {object} response.Response "Module code already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules/{id} [put]
func (h *ModuleHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req dto.UpdateModuleRequest

	// Bind request body
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	result, err := h.moduleService.Update(id, &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			response.Error(c, http.StatusNotFound, "Module not found", "")
			return
		}
		if errors.Is(err, service.ErrDuplicateModuleCode) {
			response.Error(c, http.StatusConflict, "Module code already exists", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update module", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Module updated successfully", result)
}

// Delete soft deletes a module
// @Summary Delete module
// @Description Soft delete a module (admin only)
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID"
// @Success 200 {object} response.Response "Module deleted successfully"
// @Failure 404 {object} response.Response "Module not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules/{id} [delete]
func (h *ModuleHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Get user ID from context
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	// Call service
	err = h.moduleService.Delete(id, userID)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			response.Error(c, http.StatusNotFound, "Module not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete module", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Module deleted successfully", nil)
}

// Restore restores a soft-deleted module
// @Summary Restore module
// @Description Restore a soft-deleted module (admin only)
// @Tags Modules
// @Accept json
// @Produce json
// @Param id path string true "Module ID"
// @Success 200 {object} response.Response "Module restored successfully"
// @Failure 404 {object} response.Response "Module not found or not deleted"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules/{id}/restore [post]
func (h *ModuleHandler) Restore(c *gin.Context) {
	id := c.Param("id")

	// Call service
	err := h.moduleService.Restore(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Module not found or not deleted", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Module restored successfully", nil)
}

// GetTree gets module tree structure with separated modules and sub-modules
// @Summary Get module tree structure
// @Description Get all active modules separated into root modules and sub-modules arrays, ordered by depth and sort_order
// @Tags Modules
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=dto.ModuleTreeResponse} "Module tree retrieved successfully"
// @Failure 500 {object} response.Response "Internal server error"
// @Security BearerAuth
// @Router /modules/tree [get]
func (h *ModuleHandler) GetTree(c *gin.Context) {
	result, err := h.moduleService.GetTree()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch module tree", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Module tree retrieved successfully", result)
}
