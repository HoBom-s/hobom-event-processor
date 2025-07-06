package dlq

import (
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, redisDLQ *redis.RedisDLQStore, pub publisher.KafkaPublisher) {
	service := NewService(redisDLQ, pub)
	handler := NewHandler(service)

	dlq := router.Group(poller.HoBomEventPrcessorInternalApiPrefix + "/dlq")
	{
		dlq.GET("", handler.GetDLQS)
		dlq.GET("/:key", handler.GetDLQ)
		dlq.POST("/retry/:key", handler.RetryDLQ)
	}
}
