package dlq

import (
	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func RegisterRoutes(router *gin.Engine, redisDLQ *redis.RedisDLQStore, pub publisher.KafkaPublisher, conn *grpc.ClientConn) {
	service := NewService(redisDLQ, pub, outboxPb.NewPatchOutboxControllerClient(conn))
	handler := NewHandler(service)

	dlq := router.Group(poller.HoBomEventPrcessorInternalApiPrefix + "/dlq")
	{
		dlq.GET("", handler.GetDLQS)
		dlq.GET("/:key", handler.GetDLQ)
		dlq.POST("/retry/:key", handler.RetryDLQ)
	}
}
