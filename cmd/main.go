package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcConn "github.com/HoBom-s/hobom-event-processor/infra/grpc"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/internal/dlq"
	"github.com/HoBom-s/hobom-event-processor/internal/health"
	"github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	_ = godotenv.Load()

	// 1. Connect gRPC
	grpcAddr := mustEnv("HOBOM_GRPC_ADDR")
	grpcApiKey := mustEnv("HOBOM_GRPC_API_KEY")

	conn, err := grpcConn.NewConn(grpcAddr, grpcApiKey)
	if err != nil {
		slog.Error("failed to connect to gRPC", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 1-1. Connect space gRPC (optional — hobom-space-backend)
	var spaceConn *grpc.ClientConn
	if spaceAddr := os.Getenv("HOBOM_SPACE_GRPC_ADDR"); spaceAddr != "" {
		spaceApiKey := envOrDefault("HOBOM_SPACE_GRPC_API_KEY", grpcApiKey)
		spaceConn, err = grpcConn.NewConn(spaceAddr, spaceApiKey)
		if err != nil {
			slog.Error("failed to connect to space gRPC", "err", err)
			os.Exit(1)
		}
		defer spaceConn.Close()
		slog.Info("space gRPC connected", "addr", spaceAddr)
	}

	// 1-2. Connect LLM gRPC (optional — hobom-llm-service-backend)
	var llmConn *grpc.ClientConn
	if llmAddr := os.Getenv("HOBOM_LLM_GRPC_ADDR"); llmAddr != "" {
		llmApiKey := envOrDefault("HOBOM_LLM_GRPC_API_KEY", grpcApiKey)
		llmConn, err = grpcConn.NewConn(llmAddr, llmApiKey)
		if err != nil {
			slog.Error("failed to connect to LLM gRPC", "err", err)
			os.Exit(1)
		}
		defer llmConn.Close()
		slog.Info("LLM gRPC connected", "addr", llmAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. KafkaPublisher 생성
	kafkaBroker := mustEnv("HOBOM_KAFKA_BROKER")
	kafkaPublisher := publisher.NewKafkaPublisher(publisher.DefaultKafkaConfig([]string{kafkaBroker}))

	// 3. RedisClient 생성
	redisAddr := mustEnv("HOBOM_REDIS_ADDR")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	rc := redisClient.NewRedisDLQStore(rdb)

	// 4. Start polling ( Background )
	wg := poller.StartAllPollers(ctx, conn, spaceConn, llmConn, kafkaPublisher, rc)

	// 5. Start Gin server
	internalApiKey := os.Getenv("HOBOM_INTERNAL_API_KEY")
	router := gin.Default()
	health.RegisterRoutes(router, rdb, conn, spaceConn)
	dlq.RegisterRoutes(router, rc, kafkaPublisher, conn, spaceConn, llmConn, internalApiKey)
	httpAddr := envOrDefault("HOBOM_HTTP_ADDR", ":8082")
	server := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

	go func() {
		slog.Info("HTTP server starting", "addr", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
			os.Exit(1)
		}
	}()

	// 6. Listen OS Signal for Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	slog.Info("shutdown signal received")

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all pollers stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("poller shutdown timed out after 10s, forcing exit")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "err", err)
	}

	slog.Info("shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
