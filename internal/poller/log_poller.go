package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	outboxFindPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/log/outbox/v1"
	outboxPatchPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type logPoller struct {
	findClient  outboxFindPb.FindHoBomLogOutboxControllerClient
	patchClient outboxPatchPb.PatchOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewLogPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &logPoller{
		findClient:  outboxFindPb.NewFindHoBomLogOutboxControllerClient(conn),
		patchClient: outboxPatchPb.NewPatchOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// Poll fetches PENDING log outbox events from for-hobom-backend and publishes
// them as a single batched JSON array to the "hobom.logs" Kafka topic.
//
// Flow:
//
//	gRPC FindLogOutbox(HOBOM_LOG, PENDING)
//	  └─ for each item:
//	       ├─ convert payload to HoBomLogMessageCommand
//	       │   └─ fail → markAsFailed, skip this item
//	       └─ marshal individual entry (for DLQ fallback)
//	           └─ fail → markAsFailed, skip this item
//	  └─ if entries collected:
//	       ├─ marshal all commands as JSON array
//	       ├─ publishWithRetry to Kafka (key: "hobom-log-<nanos>")
//	       │   └─ fail → markAsFailed + saveDLQ for each entry
//	       └─ markAsSent for each entry
//
// Batching rationale: Log events are high-volume, low-priority. Batching
// reduces Kafka round-trips. Each entry is also serialized individually so
// that DLQ retry can republish a single event without the full batch.
func (p *logPoller) Poll(ctx context.Context) error {
	req := &outboxFindPb.Request{
		EventType: EventTypeHoBomLog.String(),
		Status:    OutboxPending.String(),
	}

	res, err := p.findClient.FindLogOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch log outbox: %w", err)
	}

	type logEntry struct {
		eventId           string
		cmd               HoBomLogMessageCommand
		individualPayload []byte
	}

	var entries []logEntry
	for _, item := range res.Items {
		payloadMap, err := structToMap(item.Payload)
		if err != nil {
			p.markAsFailed(ctx, item.EventId, "failed to convert payload to map")
			continue
		}

		path := item.Payload.Path
		cmd := HoBomLogMessageCommand{
			ServiceType: item.Payload.ServiceType,
			Level:       item.Payload.Level,
			TraceId:     item.Payload.TraceId,
			Message:     item.Payload.Message,
			HttpMethod:  item.Payload.Method,
			Path:        &path,
			StatusCode:  int(item.Payload.StatusCode),
			Host:        item.Payload.Host,
			UserId:      item.Payload.UserId,
			Payload:     payloadMap,
		}

		// Each event is also serialized as a single-element array so that
		// DLQ retry consumers receive the same format as the batch publish.
		individualPayload, err := json.Marshal([]HoBomLogMessageCommand{cmd})
		if err != nil {
			p.markAsFailed(ctx, item.EventId, fmt.Sprintf("marshal error: %v", err))
			continue
		}

		entries = append(entries, logEntry{
			eventId:           item.EventId,
			cmd:               cmd,
			individualPayload: individualPayload,
		})
	}

	if len(entries) == 0 {
		return nil
	}

	// Batch all log commands into a single Kafka message.
	commands := make([]HoBomLogMessageCommand, len(entries))
	for i, e := range entries {
		commands[i] = e.cmd
	}

	jsonArray, err := json.Marshal(commands)
	if err != nil {
		slog.Error("failed to marshal log batch", "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("marshal error: %v", err))
		}
		return nil
	}

	// Use timestamp-based key for partition distribution across brokers.
	err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       fmt.Sprintf("hobom-log-%d", time.Now().UnixNano()),
		Value:     jsonArray,
		Topic:     HoBomLog,
		Timestamp: time.Now(),
	})
	if err != nil {
		slog.Error("kafka publish failed for log batch", "count", len(entries), "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("publish error: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomLogDLQPrefix, e.eventId, e.individualPayload)
		}
		return nil
	}

	for _, e := range entries {
		if err := p.markAsSent(ctx, e.eventId); err != nil {
			slog.Warn("published but failed to mark log as SENT", "eventId", e.eventId, "err", err)
		}
	}
	return nil
}

func (p *logPoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking log outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPatchPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark log outbox as SENT: %w", err)
	}
	return nil
}

func (p *logPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPatchPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark log outbox as FAILED", "eventId", eventId, "err", err)
	}
}
