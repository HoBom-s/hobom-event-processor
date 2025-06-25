package poller

import (
	"context"
	"fmt"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/menu/outbox/v1"
	"google.golang.org/grpc"
)

type todayMenuPoller struct {
	findClient  outboxPb.FindTodayMenuOutboxControllerClient
	patchClient outboxPb.PatchOutboxControllerClient
}

func NewTodayMenuPoller(conn *grpc.ClientConn) Poller {
	return &todayMenuPoller{
		findClient:  outboxPb.NewFindTodayMenuOutboxControllerClient(conn),
		patchClient: outboxPb.NewPatchOutboxControllerClient(conn),
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
// EventType 이 `TODAY_MENU` 이고, Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *todayMenuPoller) poll(ctx context.Context) {
	req := &outboxPb.Request{
		EventType: EventTypeTodayMenu,
		Status:    OutboxPending,
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return
	}

	for _, item := range res.Items {
		fmt.Printf("📥 Got Outbox ID: %s, Event: %s\n", item.Id, item.EventType)

		if publishToKafka(item) {
			p.markAsSent(ctx, item.EventId)
		} else {
			p.markAsFailed(ctx, item.EventId, "Kafka publish failed")
		}
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


// TODO: Kafka
func publishToKafka(item *outboxPb.QueryResult) bool {
	fmt.Printf("🚀 Publishing to Kafka: %s\n", item.EventId)
	return true
}
