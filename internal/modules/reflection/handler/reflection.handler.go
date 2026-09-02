package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/reflection/dto"
	"venturo-skeleton-go/internal/modules/reflection/service"
	"venturo-skeleton-go/internal/shared/response"
	"venturo-skeleton-go/pkg/logger"
)

type ReflectionHandler struct {
	svc *service.Service
}

func NewReflectionHandler(svc *service.Service) *ReflectionHandler {
	return &ReflectionHandler{svc: svc}
}

// Create starts a new reflection session for the authenticated user.
func (h *ReflectionHandler) Create(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req dto.CreateReflectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}

	req.Thought = strings.TrimSpace(req.Thought)
	if req.Thought == "" {
		response.Error(c, http.StatusBadRequest, "Input tidak valid", "")
		return
	}

	result, err := h.svc.RunReflection(c.Request.Context(), userID, req.Thought)
	if err != nil {
		logger.Error("Reflection create failed", logger.String("user_id", userID), logger.Err(err))
		switch {
		case errors.Is(err, service.ErrMissingCredentials):
			response.Error(c, http.StatusServiceUnavailable, "Kredensial Groq belum diatur. Tambahkan lewat menu Settings.", "")
		// The service joins ErrAllCredentialsFailed onto the last failure, so the
		// specific LLM-output errors must be matched first or they never surface.
		case errors.Is(err, service.ErrClassificationFailed), errors.Is(err, service.ErrDialogFailed):
			response.Error(c, http.StatusBadGateway, "Gagal memproses pikiran kamu, coba lagi", "")
		case errors.Is(err, service.ErrAllCredentialsFailed):
			response.Error(c, http.StatusServiceUnavailable, "Semua kredensial Groq sedang limit/kadaluarsa. Coba lagi nanti atau ganti key di Settings.", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Gagal memproses pikiran kamu, coba lagi", "")
		}
		return
	}

	response.Success(c, http.StatusCreated, "Refleksi berhasil dibuat", result)
}

// ContinueConversation handles continuing an interactive conversation.
func (h *ReflectionHandler) ContinueConversation(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	reflectionID := c.Param("id")

	var req dto.ContinueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}

	req.UserMessage = strings.TrimSpace(req.UserMessage)
	if req.UserMessage == "" {
		response.Error(c, http.StatusBadRequest, "Pesan tidak boleh kosong", "")
		return
	}

	result, err := h.svc.ContinueConversation(c.Request.Context(), reflectionID, userID, req.UserMessage)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMissingCredentials):
			response.Error(c, http.StatusServiceUnavailable, "Kredensial Groq belum diatur.", "")
		case errors.Is(err, service.ErrReflectionNotFound):
			response.Error(c, http.StatusNotFound, "Refleksi tidak ditemukan", "")
		case errors.Is(err, service.ErrSafetyTriggered):
			response.Error(c, http.StatusConflict, "Refleksi ini tidak bisa dilanjutkan. Silakan hubungi bantuan profesional atau mulai refleksi baru.", "")
		case errors.Is(err, service.ErrConversationMaxed):
			response.Error(c, http.StatusConflict, "Percakapan sudah mencapai batas maksimal. Mulai refleksi baru ya.", "")
		case errors.Is(err, service.ErrDialogFailed):
			response.Error(c, http.StatusBadGateway, "Gagal membuat balasan dialog. Silakan coba lagi.", "")
		case errors.Is(err, service.ErrAllCredentialsFailed):
			response.Error(c, http.StatusServiceUnavailable, "Layanan Groq sementara unavailable. Silakan coba lagi nanti.", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Gagal memproses respons", "")
		}
		return
	}

	response.Success(c, http.StatusOK, "Respons berhasil dibuat", result)
}

// List returns a paginated list of the authenticated user's reflections.
func (h *ReflectionHandler) List(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var query dto.ReflectionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	query.Defaults()

	reflections, total, err := h.svc.List(c.Request.Context(), userID, query.Page, query.Limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Gagal mengambil daftar refleksi", "")
		return
	}

	summaries := make([]dto.ReflectionSummary, 0, len(reflections))
	for _, r := range reflections {
		summaries = append(summaries, dto.ReflectionSummary{
			ID:                r.ID,
			Thought:           r.Thought,
			SafetyTriggered:   r.SafetyTriggered,
			ConversationState: r.ConversationState,
			TotalTurns:        r.TotalTurns,
			CreatedAt:         r.CreatedAt,
		})
	}

	response.SuccessWithPagination(c, http.StatusOK, "Daftar refleksi", summaries, query.Page, query.Limit, total)
}

// Get returns the full detail of a single reflection for the authenticated user.
func (h *ReflectionHandler) Get(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	id := c.Param("id")

	result, err := h.svc.Get(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, service.ErrReflectionNotFound) {
			response.Error(c, http.StatusNotFound, "Refleksi tidak ditemukan", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Gagal mengambil refleksi", "")
		return
	}

	response.Success(c, http.StatusOK, "Refleksi ditemukan", result)
}
