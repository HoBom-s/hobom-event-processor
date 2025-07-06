package main

import (
	"context"
	"log"
	"net/http"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
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

	// 4. Start polling
	go poller.StartAllPollers(ctx, conn, kafkaPublisher, rc)
	log.Printf("Started Polling...")

	// 5. Start Gin server
	router := gin.Default()
	health.RegisterRoutes(router)

	// 6. Run Gin as HTTP server
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start Gin server: %v", err)
	}
}
