package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	logPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/log/outbox/v1"
	spacePb "github.com/HoBom-s/hobom-event-processor/infra/grpc/space/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type spaceLogPoller struct {
	findClient  logPb.FindHoBomLogOutboxControllerClient
	patchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewSpaceLogPoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &spaceLogPoller{
		findClient:  logPb.NewFindHoBomLogOutboxControllerClient(conn),
		patchClient: spacePb.NewPatchHoBomSpaceOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// gRPC 통신을 통해 hobom-space-backend 서버의 Outbox DB를 polling 하도록 한다.
// hobom-space-backend의 API 요청/응답 로그를 수집하여 hobom.logs Kafka topic으로 발행한다.
// 기존 log/outbox/v1 proto를 재사용하며, EventType이 `SPACE_LOG`이고 Status가 `PENDING`인 것을 가져온다.
func (p *spaceLogPoller) Poll(ctx context.Context) error {
	req := &logPb.Request{
		EventType: EventTypeSpaceLog.String(),
		Status:    OutboxPending.String(),
	}

	res, err := p.findClient.FindLogOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch space log outbox: %w", err)
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
		slog.Error("failed to marshal space log batch", "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("marshal error: %v", err))
		}
		return nil
	}

	err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       fmt.Sprintf("space-log-%d", time.Now().UnixNano()),
		Value:     jsonArray,
		Topic:     HoBomLog,
		Timestamp: time.Now(),
	})
	if err != nil {
		slog.Error("kafka publish failed for space log batch", "count", len(entries), "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("publish error: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomSpaceLogDLQPrefix, e.eventId, e.individualPayload)
		}
		return nil
	}

	for _, e := range entries {
		if err := p.markAsSent(ctx, e.eventId); err != nil {
			slog.Warn("published but failed to mark space log as SENT", "eventId", e.eventId, "err", err)
		}
	}
	return nil
}

func (p *spaceLogPoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking space log outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &spacePb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark space log outbox as SENT: %w", err)
	}
	return nil
}

func (p *spaceLogPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &spacePb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark space log outbox as FAILED", "eventId", eventId, "err", err)
	}
}
