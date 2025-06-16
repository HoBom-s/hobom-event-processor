package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HealthCheck(context *gin.Context) {
	result := h.service.Check(context.Request.Context())
	context.JSON(http.StatusOK, result)
}