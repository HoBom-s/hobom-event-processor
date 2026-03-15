package dlq

import (
	lawPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/v1"
	llmPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/llm/v1"
	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	spacePb "github.com/HoBom-s/hobom-event-processor/infra/grpc/space/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/internal/middleware"
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func RegisterRoutes(router *gin.Engine, redisDLQ *redis.RedisDLQStore, pub publisher.KafkaPublisher, conn *grpc.ClientConn, spaceConn *grpc.ClientConn, llmConn *grpc.ClientConn, apiKey string) {
	var spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	if spaceConn != nil {
		spacePatchClient = spacePb.NewPatchHoBomSpaceOutboxControllerClient(spaceConn)
	}

	var llmClient llmPb.StudyMaterialServiceClient
	var saveClient lawPb.SaveStudyMaterialControllerClient
	if llmConn != nil {
		llmClient = llmPb.NewStudyMaterialServiceClient(llmConn)
		saveClient = lawPb.NewSaveStudyMaterialControllerClient(conn)
	}

	service := NewService(redisDLQ, pub, outboxPb.NewPatchOutboxControllerClient(conn), spacePatchClient, llmClient, saveClient)
	handler := NewHandler(service)

	dlq := router.Group(poller.HoBomEventProcessorInternalApiPrefix + "/dlq")
	dlq.Use(middleware.APIKeyAuth(apiKey))
	{
		dlq.GET("", handler.GetDLQS)
		dlq.GET("/:key", handler.GetDLQ)
		dlq.POST("/retry/:key", handler.RetryDLQ)
	}
}
