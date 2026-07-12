package health

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

// RegisterRoutes mounts the health check endpoint at GET /health.
// This endpoint is NOT protected by API key auth — it must be publicly
// accessible for container orchestration health probes.
func RegisterRoutes(router *gin.Engine, redisClient *redis.Client, grpcConn *grpc.ClientConn, spaceConn *grpc.ClientConn) {
	service := NewService(redisClient, grpcConn, spaceConn)
	handler := NewHandler(service)

	router.GET("/health", handler.HealthCheck)
}
