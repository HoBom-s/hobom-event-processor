package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/log/outbox/v1"
	outboxMenuPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/menu/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type logPoller struct {
	findClient	outboxPb.FindHoBomLogOutboxControllerClient
	patchClient outboxMenuPb.PatchOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ 	*redisClient.RedisDLQStore
}

func NewLogPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ *redisClient.RedisDLQStore) Poller {
	return &logPoller{
		findClient: outboxPb.NewFindHoBomLogOutboxControllerClient(conn),
		patchClient: outboxMenuPb.NewPatchOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:	 redisDLQ,
	}
}

func (p *logPoller) StartPolling(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				p.poll(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// gRPC 통신을 통한 for-hobom-backend 서버의 Outbox DB 를 polling 하도록 한다.
// 해당 서버를 통과한 API 요청 및 응답에 대한 Log 들을 수집하고, hobom-internal-backend 로 적재하기 위한 데이터를 가지고 있다.
// EventType이 `HOBOM_LOG` 이고, Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *logPoller) poll(ctx context.Context) {
	req := &outboxPb.Request{
		EventType: EventTypeHoBomLog,
		Status:    OutboxPending,
	}

	res, err := p.findClient.FindLogOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed to fetch outbox: %v\n", err)
		return
	}

	var commands []HoBomLogMessageCommand
	var eventIds []string

	for _, item := range res.Items {
		fmt.Println(item.Payload.TraceId)

		payloadMap, err := structToMap(item.Payload)
		if err != nil {
			p.markAsFailed(ctx, item.EventId, "Failed to convert payload to map")
			continue
		}

		cmd := HoBomLogMessageCommand{
			ServiceType: item.Payload.ServiceType,
			Level:       item.Payload.Level,
			TraceId:     item.Payload.TraceId,
			Message:     item.Payload.Message,
			HttpMethod:  item.Payload.Method,
			Path:        &item.Payload.Path,
			StatusCode:  int(item.Payload.StatusCode),
			Host:        item.Payload.Host,
			UserId:      item.Payload.UserId,
			Payload:     payloadMap,
		}

		commands = append(commands, cmd)
		eventIds = append(eventIds, item.EventId)
	}

	// Event를 발행할 Commands (Log)의 길이가 0 일 경우, 아무런 동작도
	// 수행하지 않도록 한다.
	if len(commands) == 0 {
		return
	}

	jsonArray, err := json.Marshal(commands)
	if err != nil {
		fmt.Printf("❌ Failed to marshal JSON array: %v\n", err)
		for _, id := range eventIds {
			p.markAsFailed(ctx, id, fmt.Sprintf("marshal error: %v", err))
		}
		return
	}

	err = p.publisher.Publish(ctx, publisher.Event{
		Key:       "hobom-log-batch",
		Value:     jsonArray,
		Topic:     HoBomLog,
		Timestamp: time.Now(),
	})
	// Kafka Event발행에 실패했을 경우, gRPC를 통해 Outbox 데이터를 Fail 로 업데이트 하도록 한다.
	// 그 후, Redis에 DLQ Event를 저장하도록 한다.
	if err != nil {
		fmt.Printf("❌ Kafka publish failed: %v\n", err)
		for _, id := range eventIds {
			p.markAsFailed(ctx, id, fmt.Sprintf("publish error: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomLogDLQPrefix, id, jsonArray)
		}
		return
	}

	// Mark as SENT only after successful publish
	for _, id := range eventIds {
		p.markAsSent(ctx, id)
	}
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `SENT` 상태로 업데이트를 한다.
func (p *logPoller) markAsSent(ctx context.Context, eventId string) {
	fmt.Printf("📥 Marking as SENT: %s\n", eventId)
	p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxMenuPb.MarkRequest{
		EventId: eventId,
	})
}

// gRPC 통신을 통해, for-hobo-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `FAILED` 상태로 업데이트를 한다.
func (p *logPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxMenuPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	})
}
