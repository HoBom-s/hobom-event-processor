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
// Poll executes a single polling cycle and returns an error on failure.
type Poller interface {
	Poll(ctx context.Context) error
}

// StartAllPollers starts all pollers in background goroutines and returns a WaitGroup.
// Callers must cancel ctx then call wg.Wait() to ensure all in-flight poll cycles complete
// before shutting down.
// spaceConn is optional — if nil, the space poller is not started.
// llmConn is optional — if nil, the law poller is not started.
func StartAllPollers(ctx context.Context, conn *grpc.ClientConn, spaceConn *grpc.ClientConn, llmConn *grpc.ClientConn, kafkaPublisher publisher.KafkaPublisher, dlqStore redis.DLQStore) *sync.WaitGroup {
	pollers := []Poller{
		NewMessagePoller(conn, kafkaPublisher, dlqStore),
		NewLogPoller(conn, kafkaPublisher, dlqStore),
	}
	if spaceConn != nil {
		pollers = append(pollers, NewSpacePoller(spaceConn, kafkaPublisher, dlqStore))
		pollers = append(pollers, NewSpaceLogPoller(spaceConn, kafkaPublisher, dlqStore))
	}
	if llmConn != nil {
		pollers = append(pollers, NewLawPoller(conn, llmConn, dlqStore))
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
