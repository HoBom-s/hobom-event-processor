// Package poller implements the Transactional Outbox consumer pattern.
//
// Each poller runs a periodic loop that:
//  1. Fetches PENDING outbox events from a backend service via gRPC.
//  2. Processes them (publish to Kafka).
//  3. Marks the outbox entry as SENT (success) or FAILED (error).
//  4. On failure, saves the event payload to Redis DLQ for manual retry.
//
// The polling lifecycle is managed by StartAllPollers, which launches each
// poller in its own goroutine and returns a WaitGroup for coordinated shutdown.
//
// Failure Handling:
//   - Individual event errors (marshal, publish) are handled per-event: the
//     event is marked FAILED and saved to DLQ. The poller continues processing
//     remaining events in the same cycle.
//   - gRPC fetch errors cause the entire poll cycle to fail, triggering
//     exponential backoff (5s → 10s → 20s → ... → capped at 60s). On recovery,
//     the interval resets to 5s.
package poller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redis "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

const (
	pollingInterval    = 5 * time.Second
	maxBackoffInterval = 60 * time.Second
)

// Poller is the interface implemented by all event pollers.
// Poll executes a single polling cycle and returns an error if the
// upstream gRPC call fails (triggering backoff in the run loop).
type Poller interface {
	Poll(ctx context.Context) error
}

// StartAllPollers launches all pollers as background goroutines.
//
// Poller selection is based on available gRPC connections:
//   - MessagePoller and LogPoller always start (use the main conn).
//   - SpacePoller and SpaceLogPoller start only when spaceConn != nil.
//
// Returns a WaitGroup that completes when all pollers have exited.
// Callers should cancel ctx, then call wg.Wait() to ensure in-flight
// poll cycles finish before shutting down shared resources (gRPC, Kafka).
func StartAllPollers(ctx context.Context, conn *grpc.ClientConn, spaceConn *grpc.ClientConn, angelConn *grpc.ClientConn, kafkaPublisher publisher.KafkaPublisher, dlqStore redis.DLQStore) *sync.WaitGroup {
	pollers := []Poller{
		NewMessagePoller(conn, kafkaPublisher, dlqStore),
		NewLogPoller(conn, kafkaPublisher, dlqStore),
	}
	if spaceConn != nil {
		pollers = append(pollers, NewSpacePoller(spaceConn, kafkaPublisher, dlqStore))
		pollers = append(pollers, NewSpaceLogPoller(spaceConn, kafkaPublisher, dlqStore))
	}
	if angelConn != nil {
		pollers = append(pollers, NewAngelPoller(angelConn, kafkaPublisher, dlqStore))
		pollers = append(pollers, NewAngelLogPoller(angelConn, kafkaPublisher, dlqStore))
	}

	var wg sync.WaitGroup
	for _, p := range pollers {
		wg.Add(1)
		p := p
		go func() {
			defer wg.Done()
			runPoller(ctx, p)
		}()
	}

	slog.Info("all pollers started")
	return &wg
}

// runPoller drives a single poller's lifecycle:
//
//	┌─────────────────────────────────────────────────────┐
//	│  ticker fires (default 5s)                          │
//	│  ├─ create 30s timeout context                      │
//	│  ├─ call p.Poll(ctx)                                │
//	│  │   ├─ success → reset ticker to 5s                │
//	│  │   └─ error   → increment failure counter         │
//	│  │               → backoff = 5s × 2^failures        │
//	│  │               → cap at 60s, reset ticker          │
//	│  └─ log traceId for cycle correlation               │
//	│                                                     │
//	│  ctx.Done() → exit                                  │
//	└─────────────────────────────────────────────────────┘
func runPoller(ctx context.Context, p Poller) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-ticker.C:
			traceID := fmt.Sprintf("%x", time.Now().UnixNano())
			slog.Debug("poll cycle start", "traceId", traceID)

			pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
			err := p.Poll(pollCtx)
			pollCancel()

			if err != nil {
				consecutiveFailures++
				backoff := pollingInterval * time.Duration(1<<min(consecutiveFailures, 4))
				if backoff > maxBackoffInterval {
					backoff = maxBackoffInterval
				}
				slog.Error("poll cycle failed", "traceId", traceID, "consecutiveFailures", consecutiveFailures, "nextBackoff", backoff, "err", err)
				ticker.Reset(backoff)
			} else {
				if consecutiveFailures > 0 {
					slog.Info("poll cycle recovered", "traceId", traceID, "previousFailures", consecutiveFailures)
				}
				consecutiveFailures = 0
				ticker.Reset(pollingInterval)
			}

			slog.Debug("poll cycle end", "traceId", traceID)
		case <-ctx.Done():
			return
		}
	}
}
