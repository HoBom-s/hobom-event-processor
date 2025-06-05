package port

import (
	outboxEvent "github.com/HoBom-s/hobom-event-processor/internal/port/out/poller"
)

type OutboxEventPublisher interface {
	Publish(event outboxEvent.OutboxEvent) error
}