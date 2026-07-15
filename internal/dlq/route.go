package dlq

import (
	angelPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/angel/outbox/v1"
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

// RegisterRoutes mounts the DLQ management endpoints under
// /hobom-event-processor/internal/api/v1/dlq.
//
// All endpoints are protected by APIKeyAuth middleware. When apiKey is empty
// (local dev), auth is skipped.
//
// gRPC clients are created from the provided connections. spaceConn and
// llmConn may be nil — in that case, DLQ retry for space/law events will
// return an error explaining the missing connection.
func RegisterRoutes(router *gin.Engine, redisDLQ *redis.RedisDLQStore, pub publisher.KafkaPublisher, conn *grpc.ClientConn, spaceConn *grpc.ClientConn, llmConn *grpc.ClientConn, angelConn *grpc.ClientConn, apiKey string) {
	var spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	if spaceConn != nil {
		spacePatchClient = spacePb.NewPatchHoBomSpaceOutboxControllerClient(spaceConn)
	}

	var angelPatchClient angelPb.PatchHoBomAngelOutboxControllerClient
	if angelConn != nil {
		angelPatchClient = angelPb.NewPatchHoBomAngelOutboxControllerClient(angelConn)
	}

	var llmClient llmPb.StudyMaterialServiceClient
	var saveClient lawPb.SaveStudyMaterialControllerClient
	if llmConn != nil {
		llmClient = llmPb.NewStudyMaterialServiceClient(llmConn)
		saveClient = lawPb.NewSaveStudyMaterialControllerClient(conn)
	}

	service := NewService(redisDLQ, pub, outboxPb.NewPatchOutboxControllerClient(conn), spacePatchClient, angelPatchClient, llmClient, saveClient)
	handler := NewHandler(service)

	dlq := router.Group(poller.HoBomEventProcessorInternalApiPrefix + "/dlq")
	dlq.Use(middleware.APIKeyAuth(apiKey))
	{
		dlq.GET("", handler.GetDLQS)
		dlq.GET("/:key", handler.GetDLQ)
		dlq.POST("/retry/:key", handler.RetryDLQ)
	}
}
