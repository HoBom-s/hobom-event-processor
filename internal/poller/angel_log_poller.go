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
)

// angelLogServiceType tags Angel access logs in the shared "hobom.logs" stream.
// Must match hobom-internal-backend's ServiceType enum (HOBOM_* convention).
const angelLogServiceType = "HOBOM_ANGEL"

type angelLogPoller struct {
	findClient  angelPb.FindHoBomAngelOutboxControllerClient
	patchClient angelPb.PatchHoBomAngelOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewAngelLogPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &angelLogPoller{
		findClient:  angelPb.NewFindHoBomAngelOutboxControllerClient(conn),
		patchClient: angelPb.NewPatchHoBomAngelOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// Poll fetches PENDING HOBOM_LOG events from the Angel outbox (their payload is
// the AccessLog variant) and publishes them as a batched JSON array to the
// shared "hobom.logs" Kafka topic — mirroring LogPoller/SpaceLogPoller.
func (p *angelLogPoller) Poll(ctx context.Context) error {
	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, &angelPb.Request{
		EventType: EventTypeHoBomLog.String(),
		Status:    OutboxPending.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch angel log outbox: %w", err)
	}

	type logEntry struct {
		eventId           string
		cmd               HoBomLogMessageCommand
		individualPayload []byte
	}

	var entries []logEntry
	for _, item := range res.Items {
		accessLog := item.GetPayload().GetAccessLog()
		if accessLog == nil {
			p.markAsFailed(ctx, item.EventId, "missing access_log payload")
			continue
		}

		path := accessLog.GetPath()
		cmd := HoBomLogMessageCommand{
			ServiceType: angelLogServiceType,
			Level:       accessLog.GetLevel(),
			TraceId:     accessLog.GetTraceId(),
			Message:     accessLog.GetMessage(),
			HttpMethod:  accessLog.GetMethod(),
			Path:        &path,
			StatusCode:  int(accessLog.GetStatusCode()),
			Host:        accessLog.GetHost(),
			UserId:      accessLog.GetUserId(),
			Payload:     metaToPayload(accessLog.GetMeta()),
		}

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

	commands := make([]HoBomLogMessageCommand, len(entries))
	for i, e := range entries {
		commands[i] = e.cmd
	}

	jsonArray, err := json.Marshal(commands)
	if err != nil {
		slog.Error("failed to marshal angel log batch", "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("marshal error: %v", err))
		}
		return nil
	}

	err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       fmt.Sprintf("angel-log-%d", time.Now().UnixNano()),
		Value:     jsonArray,
		Topic:     HoBomLog,
		Timestamp: time.Now(),
	})
	if err != nil {
		slog.Error("kafka publish failed for angel log batch", "count", len(entries), "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("publish error: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomAngelLogDLQPrefix, e.eventId, e.individualPayload)
		}
		return nil
	}

	for _, e := range entries {
		if err := p.markAsSent(ctx, e.eventId); err != nil {
			slog.Warn("published but failed to mark angel log as SENT", "eventId", e.eventId, "err", err)
		}
	}
	return nil
}

// metaToPayload widens the AccessLog string map into the generic log payload.
func metaToPayload(meta map[string]string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func (p *angelLogPoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking angel log outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &angelPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark angel log outbox as SENT: %w", err)
	}
	return nil
}

func (p *angelLogPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &angelPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark angel log outbox as FAILED", "eventId", eventId, "err", err)
	}
}
