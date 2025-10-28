package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/chats/dto"
	"venturo-skeleton-go/internal/modules/crm/chats/service"
	"venturo-skeleton-go/internal/shared/response"
)

type ChatHandler struct {
	service *service.ChatService
}

func NewChatHandler(service *service.ChatService) *ChatHandler {
	return &ChatHandler{service: service}
}

// Create godoc
// @Summary Create new chat
// @Description Create a new chat session (for WhatsApp webhook integration)
// @Tags CRM - Chats
// @Accept json
// @Produce json
// @Param request body dto.CreateChatRequest true "Chat data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/chats [post]
func (h *ChatHandler) Create(c *gin.Context) {
	var req dto.CreateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	chat, err := h.service.Create(companyID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create chat", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Chat created successfully", chat)
}

// GetAll godoc
// @Summary Get all chats
// @Description Get list of all chat sessions with pagination and filters
// @Tags CRM - Chats
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by customer name, phone, or email"
// @Param platform query string false "Filter by platform (whatsapp, telegram, email, phone)"
// @Param status query string false "Filter by status (active, archived, closed)"
// @Param category query string false "Filter by category (hot, warm, cold)"
// @Param assigned_to_company_user_id query string false "Filter by assigned user"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/chats [get]
func (h *ChatHandler) GetAll(c *gin.Context) {
	var params dto.ChatQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	chats, total, _, err := h.service.GetAll(companyID, params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch chats", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Chats retrieved successfully", chats, params.Page, params.PageSize, int64(total))
}

// GetByIDWithMessages godoc
// @Summary Get chat detail with messages
// @Description Get a single chat with all its messages
// @Tags CRM - Chats
// @Accept json
// @Produce json
// @Param id path string true "Chat ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /crm/v1/chats/{id} [get]
func (h *ChatHandler) GetByIDWithMessages(c *gin.Context) {
	id := c.Param("id")
	companyID := middleware.GetCompanyID(c)

	if companyID == "" {
		response.Error(c, http.StatusBadRequest, "Company ID is required", "")
		return
	}

	chatDetail, err := h.service.GetByIDWithMessages(id, companyID)
	if err != nil {
		if errors.Is(err, service.ErrChatNotFound) {
			response.Error(c, http.StatusNotFound, "Chat not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to fetch chat", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Chat retrieved successfully", chatDetail)
}
