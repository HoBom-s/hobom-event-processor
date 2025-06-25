package poller

import (
	"context"
	"log"
	"sync"

	"google.golang.org/grpc"
)

// 공통 Poller 인터페이스
type Poller interface {
	StartPolling(ctx context.Context)
}

// 모든 polling 을 초기화 및 수행하도록 한다.
// gRPC 통신을 위한 초기 로직을 수행하도록 한다.
func StartAllPollers(ctx context.Context, conn *grpc.ClientConn) {
	var wg sync.WaitGroup

	pollers := []Poller{
		NewTodayMenuPoller(conn),
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
