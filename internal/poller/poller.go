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
func StartAllPollers(ctx context.Context, conn *grpc.ClientConn, kafkaPublisher publisher.KafkaPublisher, dlqStore redis.DLQStore) *sync.WaitGroup {
	pollers := []Poller{
		NewMessagePoller(conn, kafkaPublisher, dlqStore),
		NewLogPoller(conn, kafkaPublisher, dlqStore),
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
					p.Poll(ctx)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	slog.Info("all pollers started")
	return &wg
}
