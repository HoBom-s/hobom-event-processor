package http

import (
	"fmt"

	healthRouter "github.com/HoBom-s/hobom-event-processor/internal/infra/http/health"
	healthService "github.com/HoBom-s/hobom-event-processor/internal/service/health"
	"github.com/gin-gonic/gin"
)

func StartHTTPServer(port int) {
	router := gin.Default()

	healthService := healthService.NewHealthService()
	healthRouter := healthRouter.CreateHealthRouter(healthService)

	hobomRouter := CreateRoute(healthRouter)
	hobomRouter.RegisterRoutes(router)

	address := fmt.Sprintf(":%d", port)
	if err := router.Run(address); err != nil {
		panic(fmt.Sprintf("HoBom Event Processor Failed... : %v", err))
	}
}