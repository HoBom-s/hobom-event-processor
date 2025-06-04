package health

import (
	"net/http"

	service "github.com/HoBom-s/hobom-event-processor/internal/service/health"
	"github.com/gin-gonic/gin"
)

type HealthRouter struct {
	healthService *service.HealthService
}


func CreateHealthRouter(healthService *service.HealthService) *HealthRouter {
	return &HealthRouter{
		healthService: healthService,
	}
}

func (h *HealthRouter) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/health", func(c *gin.Context) {
		status := h.healthService.Check()
		c.String(http.StatusOK, status)
	})
}