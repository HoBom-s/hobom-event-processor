package main

import (
	"context"
	"log"
	"net/http"

	"github.com/HoBom-s/hobom-event-processor/internal/health"
	"github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	// 1. Connect gRPC
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start polling
	p := poller.NewPoller(conn)
	p.StartPolling(ctx)
	log.Printf("Started Polling...")

	// 3. Start Gin server
	router := gin.Default()
	health.RegisterRoutes(router)

	// 4. Run Gin as HTTP server
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start Gin server: %v", err)
	}
}
