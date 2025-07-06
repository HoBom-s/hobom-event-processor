package main

import (
	"context"
	"log"
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
	// 1. Connect gRPC
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. KafkaPublisher 생성
	kafkaCfg := publisher.KafkaConfig{
		Brokers:      []string{"localhost:9092"},
	}
	kafkaPublisher := publisher.NewKafkaPublisher(kafkaCfg)

	// 3. RedisClient 생성
	rc := redisClient.NewRedisDLQStore(
			redis.NewClient(&redis.Options{
			Addr: 		"localhost:6379",
			Password: 	"",
			DB: 		0,
		}),
	)

	// 4. Start polling ( Background )
	// Background process using Goroutine
	go poller.StartAllPollers(ctx, conn, kafkaPublisher, rc)
	log.Printf("✅ Started Polling...")

	// 5. Start Gin server
	// 5-1. Health Router
	// 5-1. DLQ Router
	router := gin.Default()
	health.RegisterRoutes(router)
	dlq.RegisterRoutes(router, rc, kafkaPublisher)
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}
	
	// 6. Setup Graceful Shutdown
	// Main Thred 차단 방지
	go func() {
		log.Println("🚀 Starting HTTP server on :8081")
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start Gin server: %v", err)
		}
	}()

	// 7. Listen OS Signal for Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("📦 Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ HTTP Server Shutdown Failed: %v", err)
	}

	log.Println("🧼 Cleanup completed. Bye!")
}
