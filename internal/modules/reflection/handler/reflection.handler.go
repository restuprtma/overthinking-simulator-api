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
		switch {
		case errors.Is(err, service.ErrMissingCredentials):
			response.Error(c, http.StatusServiceUnavailable, "Kredensial Gemini belum diatur. Tambahkan lewat menu Settings.", "")
		case errors.Is(err, service.ErrAllCredentialsFailed):
			response.Error(c, http.StatusServiceUnavailable, "Semua kredensial Gemini sedang limit/kadaluarsa. Coba lagi nanti atau ganti key di Settings.", "")
		case errors.Is(err, service.ErrClassificationFailed), errors.Is(err, service.ErrDialogFailed):
			response.Error(c, http.StatusBadGateway, "Gagal memproses pikiran kamu, coba lagi", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Gagal memproses pikiran kamu, coba lagi", "")
		}
		return
	}

	response.Success(c, http.StatusCreated, "Refleksi berhasil dibuat", result)
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
			ID:              r.ID,
			Thought:         r.Thought,
			SafetyTriggered: r.SafetyTriggered,
			CreatedAt:       r.CreatedAt,
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
