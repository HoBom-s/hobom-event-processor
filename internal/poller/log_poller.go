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

// gRPC 통신을 통한 for-hobom-backend 서버의 Outbox DB 를 polling 하도록 한다.
// 해당 서버를 통과한 API 요청 및 응답에 대한 Log 들을 수집하고, hobom-internal-backend 로 적재하기 위한 데이터를 가지고 있다.
// EventType이 `HOBOM_LOG` 이고, Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *logPoller) Poll(ctx context.Context) {
	req := &outboxFindPb.Request{
		EventType: EventTypeHoBomLog,
		Status:    OutboxPending,
	}

	res, err := p.findClient.FindLogOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		slog.Error("failed to fetch log outbox", "err", err)
		return
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

		// 각 이벤트를 단일 원소 배열로 직렬화한다.
		// DLQ retry 시 컨슈머가 배치 발행과 동일한 포맷을 수신하도록 보장한다.
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

	// Event를 발행할 Commands (Log)의 길이가 0 일 경우, 아무런 동작도
	// 수행하지 않도록 한다.
	if len(entries) == 0 {
		return
	}

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
		return
	}

	// 파티션 분산을 위해 타임스탬프 기반 키를 사용한다.
	err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       fmt.Sprintf("hobom-log-%d", time.Now().UnixNano()),
		Value:     jsonArray,
		Topic:     HoBomLog,
		Timestamp: time.Now(),
	})
	// Kafka Event발행에 실패했을 경우, gRPC를 통해 Outbox 데이터를 Fail 로 업데이트 하도록 한다.
	// 그 후, Redis에 DLQ Event를 저장하도록 한다.
	if err != nil {
		slog.Error("kafka publish failed for log batch", "count", len(entries), "err", err)
		for _, e := range entries {
			p.markAsFailed(ctx, e.eventId, fmt.Sprintf("publish error: %v", err))
			saveDLQ(p.redisDLQ, ctx, HoBomLogDLQPrefix, e.eventId, e.individualPayload)
		}
		return
	}

	// Mark as SENT only after successful publish
	for _, e := range entries {
		p.markAsSent(ctx, e.eventId)
	}
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `SENT` 상태로 업데이트를 한다.
func (p *logPoller) markAsSent(ctx context.Context, eventId string) {
	slog.Info("marking log outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPatchPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		slog.Error("failed to mark log outbox as SENT", "eventId", eventId, "err", err)
	}
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `FAILED` 상태로 업데이트를 한다.
func (p *logPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPatchPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark log outbox as FAILED", "eventId", eventId, "err", err)
	}
}
