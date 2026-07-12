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

// HealthCheck returns the aggregated health status of all dependencies.
// Returns 200 if all components are healthy, 503 otherwise.
func (h *Handler) HealthCheck(c *gin.Context) {
	result := h.service.Check(c.Request.Context())
	status := http.StatusOK
	if result.Status != "healthy" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, result)
}
