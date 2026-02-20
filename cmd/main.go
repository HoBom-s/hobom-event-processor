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
	redis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// 1. Connect gRPC
	conn, err := grpc.NewClient("dev-for-hobom-backend:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to gRPC", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. KafkaPublisher 생성
	kafkaPublisher := publisher.NewKafkaPublisher(publisher.DefaultKafkaConfig([]string{"kafka:9092"}))

	// 3. RedisClient 생성
	rc := redisClient.NewRedisDLQStore(
		redis.NewClient(&redis.Options{
			Addr:     "redis:6379",
			Password: "",
			DB:       0,
		}),
	)

	// 4. Start polling ( Background )
	wg := poller.StartAllPollers(ctx, conn, kafkaPublisher, rc)

	// 5. Start Gin server
	router := gin.Default()
	health.RegisterRoutes(router)
	dlq.RegisterRoutes(router, rc, kafkaPublisher, conn)
	server := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	go func() {
		slog.Info("HTTP server starting", "addr", ":8082")
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
