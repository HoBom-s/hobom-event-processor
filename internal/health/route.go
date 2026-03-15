package health

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func RegisterRoutes(router *gin.Engine, redisClient *redis.Client, grpcConn *grpc.ClientConn, spaceConn *grpc.ClientConn) {
	service := NewService(redisClient, grpcConn, spaceConn)
	handler := NewHandler(service)

	router.GET("/health", handler.HealthCheck)
}
