package health

import (
	"context"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type Service interface {
	Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

type service struct {
	redisClient *redis.Client
	grpcConn    *grpc.ClientConn
	spaceConn   *grpc.ClientConn
}

func NewService(redisClient *redis.Client, grpcConn *grpc.ClientConn, spaceConn *grpc.ClientConn) Service {
	return &service{
		redisClient: redisClient,
		grpcConn:    grpcConn,
		spaceConn:   spaceConn,
	}
}

func (s *service) Check(ctx context.Context) HealthStatus {
	components := make(map[string]string)
	overall := "healthy"

	// Redis ping
	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		components["redis"] = "unhealthy"
		overall = "unhealthy"
	} else {
		components["redis"] = "healthy"
	}

	// gRPC (for-hobom-backend)
	grpcState := s.grpcConn.GetState()
	components["grpc"] = grpcState.String()
	if grpcState == connectivity.TransientFailure || grpcState == connectivity.Shutdown {
		overall = "unhealthy"
	}

	// gRPC (hobom-space-backend, optional)
	if s.spaceConn != nil {
		spaceState := s.spaceConn.GetState()
		components["grpc_space"] = spaceState.String()
		if spaceState == connectivity.TransientFailure || spaceState == connectivity.Shutdown {
			overall = "unhealthy"
		}
	}

	return HealthStatus{
		Status:     overall,
		Components: components,
	}
}
