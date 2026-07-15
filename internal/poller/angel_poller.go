package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	angelPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/angel/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// angelNotificationEventTypes are the notification-bearing Angel outbox event
// types. Each is polled separately (the Find contract filters by a single type)
// and published to hobom.angel-events. HOBOM_LOG is drained by angelLogPoller
// into hobom.logs instead.
var angelNotificationEventTypes = []EventType{
	EventTypeAngelAdoptionApproved,
	EventTypeAngelFosterApproved,
	EventTypeAngelStaffPromotionApproved,
	EventTypeAngelShelterVerificationApproved,
	EventTypeAngelFosterTerminated,
}

type angelPoller struct {
	findClient  angelPb.FindHoBomAngelOutboxControllerClient
	patchClient angelPb.PatchHoBomAngelOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewAngelPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &angelPoller{
		findClient:  angelPb.NewFindHoBomAngelOutboxControllerClient(conn),
		patchClient: angelPb.NewPatchHoBomAngelOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// Poll fetches PENDING Angel notification events from for-hobom-angel-backend
// and publishes each to the "hobom.angel-events" Kafka topic. It iterates the
// notification event types because the outbox Find filters by a single type.
//
// Flow per event: FindOutbox → build command → publish → markAsSent
// (on failure: markAsFailed + saveDLQ).
func (p *angelPoller) Poll(ctx context.Context) error {
	for _, eventType := range angelNotificationEventTypes {
		res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, &angelPb.Request{
			EventType: eventType.String(),
			Status:    OutboxPending.String(),
		})
		if err != nil {
			return fmt.Errorf("failed to fetch angel outbox (%s): %w", eventType, err)
		}
		for _, item := range res.Items {
			p.handleAngelEvent(ctx, item)
		}
	}
	return nil
}

func (p *angelPoller) handleAngelEvent(ctx context.Context, item *angelPb.QueryResult) {
	cmd := buildAngelEventCommand(item)

	jsonValue, err := json.Marshal(cmd)
	if err != nil {
		slog.Error("failed to marshal angel event payload", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("failed to marshal payload: %v", err))
		return
	}

	if err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       item.EventId,
		Value:     jsonValue,
		Topic:     HoBomAngelEvents,
		Timestamp: time.Now(),
	}); err != nil {
		slog.Error("kafka publish failed", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("kafka publish failed: %v", err))
		saveDLQ(p.redisDLQ, ctx, HoBomAngelDLQPrefix, item.EventId, jsonValue)
		return
	}

	if err := p.markAsSent(ctx, item.EventId); err != nil {
		slog.Warn("published but failed to mark angel as SENT", "eventId", item.EventId, "err", err)
	}
}

// buildAngelEventCommand flattens the discriminated AngelEventPayload union into
// a single Kafka command; consumers route on EventType. Only one of the payload
// variants is set, matching the event type.
func buildAngelEventCommand(item *angelPb.QueryResult) HoBomAngelEventCommand {
	cmd := HoBomAngelEventCommand{EventType: item.GetEventType()}
	payload := item.GetPayload()

	if a := payload.GetApprovalApproved(); a != nil {
		cmd.RecipientUserId = a.GetRecipientUserId()
		cmd.ApprovalType = a.GetApprovalType().String()
		cmd.SubjectRef = a.GetSubjectRef()
		cmd.ShelterId = a.GetShelterId()
		cmd.OccurredAt = formatAngelTimestamp(a.GetOccurredAt())
		return cmd
	}

	if f := payload.GetFosterTerminated(); f != nil {
		cmd.RecipientUserId = f.GetRecipientUserId()
		cmd.FosterProcessId = f.GetFosterProcessId()
		cmd.AnimalId = f.GetAnimalId()
		cmd.Reason = f.GetReason().String()
		cmd.OccurredAt = formatAngelTimestamp(f.GetOccurredAt())
		return cmd
	}

	return cmd
}

func formatAngelTimestamp(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}

func (p *angelPoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking angel outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &angelPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark angel outbox as SENT: %w", err)
	}
	return nil
}

func (p *angelPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &angelPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark angel outbox as FAILED", "eventId", eventId, "err", err)
	}
}
