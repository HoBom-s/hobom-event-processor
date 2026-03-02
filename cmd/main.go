package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/internal/dlq"
	"github.com/HoBom-s/hobom-event-processor/internal/health"
	"github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	_ = godotenv.Load()

	// 1. Connect gRPC
	grpcAddr := mustEnv("HOBOM_GRPC_ADDR")
	grpcApiKey := mustEnv("HOBOM_GRPC_API_KEY")

	apiKeyInterceptor := func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", grpcApiKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(apiKeyInterceptor),
	)
	if err != nil {
		slog.Error("failed to connect to gRPC", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. KafkaPublisher 생성
	kafkaBroker := mustEnv("HOBOM_KAFKA_BROKER")
	kafkaPublisher := publisher.NewKafkaPublisher(publisher.DefaultKafkaConfig([]string{kafkaBroker}))

	// 3. RedisClient 생성
	redisAddr := mustEnv("HOBOM_REDIS_ADDR")
	rc := redisClient.NewRedisDLQStore(
		redis.NewClient(&redis.Options{
			Addr: redisAddr,
		}),
	)

	// 4. Start polling ( Background )
	wg := poller.StartAllPollers(ctx, conn, kafkaPublisher, rc)

	// 5. Start Gin server
	router := gin.Default()
	health.RegisterRoutes(router)
	dlq.RegisterRoutes(router, rc, kafkaPublisher, conn)
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

	// 컨텍스트를 취소하여 폴러가 현재 poll 사이클을 완료 후 종료되도록 한다.
	cancel()
	wg.Wait()
	slog.Info("all pollers stopped")

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
