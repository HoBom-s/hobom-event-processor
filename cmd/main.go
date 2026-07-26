// hobom-event-processor is a Transactional Outbox consumer that bridges
// backend databases and Kafka pipelines.
//
// # Architecture
//
//	[for-hobom-backend]      [hobom-space-backend]
//	       │ gRPC                   │ gRPC
//	       ▼                        ▼
//	┌──────────────────────────────────────────┐
//	│         hobom-event-processor            │
//	│                                          │
//	│  Pollers (5 background goroutines)       │
//	│  ├─ MessagePoller  ──► Kafka             │
//	│  ├─ LogPoller      ──► Kafka (batch)     │
//	│  ├─ SpacePoller    ──► Kafka             │
//	│  └─ SpaceLogPoller ──► Kafka (batch)     │
//	│                                          │
//	│  DLQ Management (HTTP API, auth-gated)   │
//	│  ├─ GET  /dlq           list keys        │
//	│  ├─ GET  /dlq/:key      inspect value    │
//	│  └─ POST /dlq/retry/:key re-process      │
//	│                                          │
//	│  Health Check (HTTP API)                 │
//	│  └─ GET /health                          │
//	└──────────────────────────────────────────┘
//	       │                             │
//	       ▼                             ▼
//	   [Kafka]                       [Redis DLQ]
//
// # Startup Flow
//
//  1. Load .env, initialize structured JSON logger.
//  2. Establish gRPC connections:
//     - main conn → for-hobom-backend (required)
//     - spaceConn → hobom-space-backend (optional, env-driven)
//  3. Create Kafka publisher and Redis DLQ store.
//  4. Launch all pollers — each runs as its own goroutine with a 5s tick.
//  5. Start Gin HTTP server (health + DLQ endpoints).
//  6. Block until SIGTERM/SIGINT, then drain pollers → shut down HTTP → exit.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/internal/dlq"
	"github.com/HoBom-s/hobom-event-processor/internal/health"
	"github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	_ = godotenv.Load()

	// --- gRPC connections ---
	// Main connection is required (for-hobom-backend: outbox find/patch).
	// Space and angel connections are optional — pollers that depend on them
	// are simply not started when the env var is absent.
	grpcApiKey := mustEnv("HOBOM_GRPC_API_KEY")
	conn := mustGRPCConn(mustEnv("HOBOM_GRPC_ADDR"), grpcApiKey)
	defer conn.Close()

	spaceConn := optionalGRPCConn("HOBOM_SPACE_GRPC_ADDR", "HOBOM_SPACE_GRPC_API_KEY", grpcApiKey)
	if spaceConn != nil {
		defer spaceConn.Close()
	}

	angelConn := optionalGRPCConn("HOBOM_ANGEL_GRPC_ADDR", "HOBOM_ANGEL_GRPC_API_KEY", grpcApiKey)
	if angelConn != nil {
		defer angelConn.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Infra clients ---
	kafkaPublisher := publisher.NewKafkaPublisher(publisher.DefaultKafkaConfig([]string{mustEnv("HOBOM_KAFKA_BROKER")}))
	rdb := redis.NewClient(&redis.Options{
		Addr:     mustEnv("HOBOM_REDIS_ADDR"),
		Password: os.Getenv("HOBOM_REDIS_PASSWORD"),
	})
	rc := redisClient.NewRedisDLQStore(rdb)

	// --- Background pollers ---
	// Each poller runs a 5s-interval loop: gRPC fetch → process → Kafka → mark SENT.
	// Failed events are saved to Redis DLQ for manual retry via the HTTP API.
	wg := poller.StartAllPollers(ctx, conn, spaceConn, angelConn, kafkaPublisher, rc)

	// --- HTTP server ---
	// Health endpoint: component-level status (Redis, gRPC).
	// DLQ endpoints: inspect and retry failed events, gated by x-api-key.
	router := gin.Default()
	health.RegisterRoutes(router, rdb, conn, spaceConn)
	dlq.RegisterRoutes(router, rc, kafkaPublisher, conn, spaceConn, angelConn, os.Getenv("HOBOM_INTERNAL_API_KEY"))

	httpAddr := envOrDefault("HOBOM_HTTP_ADDR", ":8082")
	server := &http.Server{Addr: httpAddr, Handler: router}
	go func() {
		slog.Info("HTTP server starting", "addr", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
			os.Exit(1)
		}
	}()

	// --- Graceful shutdown ---
	// Blocks until OS signal, then: cancel pollers → drain with timeout → shut down HTTP.
	gracefulShutdown(cancel, wg, server)
}
