package main

import (
	"github.com/HoBom-s/hobom-event-processor/internal/health"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	health.RegisterRoutes(router)

	router.Run(":8080")
}
