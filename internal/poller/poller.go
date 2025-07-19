package poller

import (
	"context"
	"log"
	"sync"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redis "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

// 공통 Poller 인터페이스
type Poller interface {
	StartPolling(ctx context.Context)
}

// 모든 polling 을 초기화 및 수행하도록 한다.
// gRPC 통신을 위한 초기 로직을 수행하도록 한다.
// Kafka도 파라미터로 의존성을 주입받아 Event Publishing을 수행하도록 한다.
func StartAllPollers(ctx context.Context, conn *grpc.ClientConn, kafkaPublisher publisher.KafkaPublisher, redisClient *redis.RedisDLQStore) {
	var wg sync.WaitGroup

	pollers := []Poller{
		NewMessagePoller(conn, kafkaPublisher, redisClient),
		NewLogPoller(conn, kafkaPublisher, redisClient),
	}

	for _, p := range pollers {
		wg.Add(1)
		go func(poller Poller) {
			defer wg.Done()
			poller.StartPolling(ctx)
		}(p)
	}

	log.Println("🚀 All pollers started.")
	go func() {
		<-ctx.Done()
	}()

	wg.Wait()
}
