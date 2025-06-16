package health

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	service := NewService()
	handler := NewHandler(service)

	router.GET("/health", handler.HealthCheck)
}