package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	spacePb "github.com/HoBom-s/hobom-event-processor/infra/grpc/space/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type spacePoller struct {
	findClient  spacePb.FindHoBomSpaceOutboxControllerClient
	patchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewSpacePoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &spacePoller{
		findClient:  spacePb.NewFindHoBomSpaceOutboxControllerClient(conn),
		patchClient: spacePb.NewPatchHoBomSpaceOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// Poll fetches PENDING space outbox events from hobom-space-backend and
// publishes each to the "hobom.space-events" Kafka topic.
//
// Unlike LogPoller, space events are published individually (not batched)
// because each event represents a distinct user action (page create, comment
// add, etc.) that downstream consumers process independently.
//
// Flow per event:
//
//	gRPC FindOutbox(SPACE_EVENT, PENDING)
//	  └─ for each item:
//	       ├─ build HoBomSpaceEventCommand
//	       ├─ marshal → publishWithRetry → markAsSent
//	       └─ on failure: markAsFailed + saveDLQ
func (p *spacePoller) Poll(ctx context.Context) error {
	req := &spacePb.Request{
		EventType: EventTypeSpaceEvent.String(),
		Status:    OutboxPending.String(),
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch space outbox: %w", err)
	}

	for _, item := range res.Items {
		p.handleSpaceEvent(ctx, item)
	}
	return nil
}

func (p *spacePoller) handleSpaceEvent(ctx context.Context, item *spacePb.QueryResult) {
	cmd := HoBomSpaceEventCommand{
		EntityType: item.Payload.EntityType,
		Action:     item.Payload.Action,
		SpaceKey:   item.Payload.SpaceKey,
		PageId:     item.Payload.PageId,
		Title:      item.Payload.Title,
		ActorId:    item.Payload.ActorId,
	}

	jsonValue, err := json.Marshal(cmd)
	if err != nil {
		slog.Error("failed to marshal space event payload", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("failed to marshal payload: %v", err))
		return
	}

	if err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       item.EventId,
		Value:     jsonValue,
		Topic:     HoBomSpaceEvents,
		Timestamp: time.Now(),
	}); err != nil {
		slog.Error("kafka publish failed", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("kafka publish failed: %v", err))
		saveDLQ(p.redisDLQ, ctx, HoBomSpaceDLQPrefix, item.EventId, jsonValue)
		return
	}

	if err := p.markAsSent(ctx, item.EventId); err != nil {
		slog.Warn("published but failed to mark space as SENT", "eventId", item.EventId, "err", err)
	}
}

func (p *spacePoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking space outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &spacePb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark space outbox as SENT: %w", err)
	}
	return nil
}

func (p *spacePoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &spacePb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark space outbox as FAILED", "eventId", eventId, "err", err)
	}
}
