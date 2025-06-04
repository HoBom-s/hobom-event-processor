package http

import (
	healthRouter "github.com/HoBom-s/hobom-event-processor/internal/infra/http/health"
	"github.com/gin-gonic/gin"
)

type Router struct {
	healthRouter *healthRouter.HealthRouter
}

func CreateRoute(healthRouter *healthRouter.HealthRouter) *Router {
	return &Router{
		healthRouter: healthRouter,
	}
}

func (r *Router) RegisterRoutes(router *gin.Engine) {
	apiGroup := router.Group("/api")

	r.healthRouter.RegisterRoutes(apiGroup)
}