package poller

import (
	"context"
	"fmt"
	"log"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/v1/hobom-menu-outbox"

	"google.golang.org/grpc"
)


type Poller struct {
	client outboxPb.FindTodayMenuOutboxControllerClient
}

func NewPoller(conn *grpc.ClientConn) *Poller {
	return &Poller{
		client: outboxPb.NewFindTodayMenuOutboxControllerClient(conn),
	}
}

func (p *Poller) StartPolling(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				p.poll(ctx)

			case <-ctx.Done():
				log.Println("Polling stopped")
				ticker.Stop()
				return
			}
		}
	}()
}

func (p *Poller) poll(ctx context.Context) {
	req := &outboxPb.Request{
		EventType: EventTypeTodayMenu,
		Status:    OutboxPending,
	}

	res, err := p.client.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		log.Printf("gRPC call failed: %v", err)
		return
	}

	for _, item := range res.Items {
		fmt.Printf("Got Outbox ID: %s, Event: %s\n", item.Id, item.EventType)
	}
}
