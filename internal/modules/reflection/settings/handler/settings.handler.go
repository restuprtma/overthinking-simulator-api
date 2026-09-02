package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"venturo-skeleton-go/internal/modules/reflection/settings/service"
	"venturo-skeleton-go/internal/shared/response"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Get returns the current (masked) Groq credentials.
func (h *Handler) Get(c *gin.Context) {
	masked, err := h.svc.GetMaskedCredentials(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Gagal mengambil kredensial", "")
		return
	}

	response.Success(c, http.StatusOK, "OK", gin.H{"credentials": masked})
}

// Update replaces the Groq credentials list.
func (h *Handler) Update(c *gin.Context) {
	var req struct {
		Credentials []service.GroqCredential `json:"credentials" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}

	err := h.svc.SetCredentials(c.Request.Context(), req.Credentials)
	if err != nil {
		if errors.Is(err, service.ErrEmptyCredentials) {
			response.Error(c, http.StatusBadRequest, service.ErrEmptyCredentials.Error(), "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Gagal menyimpan kredensial", "")
		return
	}

	response.Success(c, http.StatusOK, "Kredensial berhasil diperbarui", nil)
}
