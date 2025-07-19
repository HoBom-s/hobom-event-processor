package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type messagePoller struct {
	findClient  outboxPb.FindHoBomMessageOutboxControllerClient
	patchClient outboxPb.PatchOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ 	*redisClient.RedisDLQStore
}

func NewMessagePoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ *redisClient.RedisDLQStore) Poller {
	return &messagePoller{
		findClient:  outboxPb.NewFindHoBomMessageOutboxControllerClient(conn),
		patchClient: outboxPb.NewPatchOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:	 redisDLQ,
	}
}

func (p *messagePoller) StartPolling(ctx context.Context) {
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

// gRPC 통신을 통해 for-hobom-backend 서버의 Outbox DB 를 polling 하도록 한다.
// Payload에는 다른 사용자에게 Message를 전송하기 위한 데이터를 가지고 있다.
// Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *messagePoller) poll(ctx context.Context) {
	req := &outboxPb.Request{
		EventType: EventTypeTodayMenu,
		Status:    OutboxPending,
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed to fetch outbox: %v\n", err)
		return
	}

	for _, item := range res.Items {
		p.handleMessage(ctx, item)
	}
}

func (p *messagePoller) handleMessage(ctx context.Context, item *outboxPb.QueryResult) {
	title := item.Payload.Title
	body := item.Payload.Body
	recipient := item.Payload.Recipient
	senderId := item.Payload.SenderId

	cmd := DeliverHoBomMessageCommand{
		Type:      Mail,
		Title:     title,
		Body:      body,
		Recipient: recipient,
		SenderId:  &senderId,
		SentAt:    time.Now(),
	}

	p.publishAndMark(ctx, item.EventId, cmd, HoBomMessage, item.EventType)
}

func (p *messagePoller) publishAndMark(
	ctx context.Context,
	eventId string,
	cmd DeliverHoBomMessageCommand,
	topic string,
	eventType string,
) {
	jsonValue, err := json.Marshal(cmd)
	if err != nil {
		p.markAsFailed(ctx, eventId, fmt.Sprintf("❌ Failed to marshal payload to JSON: %v", err))
		return
	}

	err = p.publisher.Publish(ctx, publisher.Event{
		Key:       eventId,
		Value:     jsonValue,
		Topic:     topic,
		Timestamp: time.Now(),
	})

	if err != nil {
		fmt.Printf("❌ Kafka publish failed: %v\n", err)
		p.markAsFailed(ctx, eventId, fmt.Sprintf("❌ Kafka publish failed: %v", err))
		saveDLQ(p.redisDLQ, ctx, "DLQ:"+eventType, eventId, jsonValue)
		return
	}

	p.markAsSent(ctx, eventId)
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `SENT` 상태로 업데이트를 한다.
func (p *messagePoller) markAsSent(ctx context.Context, eventId string) {
	fmt.Printf("📥 Got Outbox ID: %s", eventId)
	p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	})
}

// gRPC 통신을 통해, for-hobo-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `FAILED` 상태로 업데이트를 한다.
func (p *messagePoller) markAsFailed(ctx context.Context, eventId, reason string) {
	p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	})
}
