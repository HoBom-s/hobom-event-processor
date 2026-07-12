package health

import (
	"context"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Service defines the health check contract.
type Service interface {
	Check(ctx context.Context) HealthStatus
}

// HealthStatus is the JSON response for the /health endpoint.
// Status is "healthy" when all components are reachable, "unhealthy" otherwise.
// Components map shows individual component states for debugging.
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

// Check performs health checks against all infrastructure dependencies:
//
//  1. Redis — PING command. Unhealthy if PING fails.
//  2. gRPC (for-hobom-backend) — connection state. Unhealthy if
//     TransientFailure or Shutdown.
//  3. gRPC (hobom-space-backend) — same check, skipped if spaceConn is nil.
//
// Returns "healthy" only when ALL components are healthy.
// HTTP handler maps "unhealthy" to 503 Service Unavailable.
func (s *service) Check(ctx context.Context) HealthStatus {
	components := make(map[string]string)
	overall := "healthy"

	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		components["redis"] = "unhealthy"
		overall = "unhealthy"
	} else {
		components["redis"] = "healthy"
	}

	grpcState := s.grpcConn.GetState()
	components["grpc"] = grpcState.String()
	if grpcState == connectivity.TransientFailure || grpcState == connectivity.Shutdown {
		overall = "unhealthy"
	}

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
