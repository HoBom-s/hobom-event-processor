package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/menu/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type todayMenuPoller struct {
	findClient  outboxPb.FindTodayMenuOutboxControllerClient
	patchClient outboxPb.PatchOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ 	*redisClient.RedisDLQStore
}

func NewTodayMenuPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ *redisClient.RedisDLQStore) Poller {
	return &todayMenuPoller{
		findClient:  outboxPb.NewFindTodayMenuOutboxControllerClient(conn),
		patchClient: outboxPb.NewPatchOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:	 redisDLQ,
	}
}

func (p *todayMenuPoller) StartPolling(ctx context.Context) {
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
// 오늘의 메뉴를 추천하고, 다른 사용자에게 Message를 전송하기 위한 데이터를 가지고 있다.
// EventType 이 `TODAY_MENU` 이고, Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *todayMenuPoller) poll(ctx context.Context) {
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
		title := fmt.Sprintf("오늘의 추천 메뉴: %s", item.Payload.GetName())
		body := fmt.Sprintf("%s님이 추천한 메뉴에요.", item.Payload.GetNickname())
		recipient := item.Payload.GetEmail()
		senderId := item.Payload.GetUserId()
		sentAt := time.Now()
		cmd := DeliverHoBomMessageCommand{
			Type:      Mail,
			Title:     title,
			Body:      body,
			Recipient: recipient,
			SenderId:  &senderId,
			SentAt:    sentAt,
		}
		jsonValue, err := json.Marshal(cmd)

		if err != nil {
			p.markAsFailed(ctx, item.EventId, fmt.Sprintf("❌ Failed to marshal payload to JSON: %v", err))
			continue
		}

		err = p.publisher.Publish(ctx, publisher.Event{
			Key:       	item.EventId,
			Value:     	jsonValue,
			Topic:     	HoBomMessage,
			Timestamp: 	time.Now(),
		})
		// Kafka Event발행에 실패했을 경우, gRPC를 통해 Outbox 데이터를 Fail로 업데이트 하도록 한다.
		// Redis에 DLQ Event를 저장하도록 한다.
		if err != nil {
			fmt.Printf("❌ Kafka publish failed: %v\n", err)
			p.markAsFailed(ctx, item.EventId, fmt.Sprintf("❌ Kafka publish failed: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomTodayMenuDLQPrefix, item.EventId, jsonValue)
			continue
		}

		p.markAsSent(ctx, item.EventId)
	}
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `SENT` 상태로 업데이트를 한다.
func (p *todayMenuPoller) markAsSent(ctx context.Context, eventId string) {
	fmt.Printf("📥 Got Outbox ID: %s", eventId)
	p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	})
}

// gRPC 통신을 통해, for-hobo-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `FAILED` 상태로 업데이트를 한다.
func (p *todayMenuPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	})
}
