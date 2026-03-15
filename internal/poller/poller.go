package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redis "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

const pollingInterval = 5 * time.Second

// Poller is the interface implemented by all event pollers.
// Poll executes a single polling cycle and returns when complete.
type Poller interface {
	Poll(ctx context.Context)
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
			ticker := time.NewTicker(pollingInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
					p.Poll(pollCtx)
					pollCancel()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	slog.Info("all pollers started")
	return &wg
}
