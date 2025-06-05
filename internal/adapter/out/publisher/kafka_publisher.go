package publisher

import (
	"github.com/HoBom-s/hobom-event-processor/infra/kafka"
	outboxEvent "github.com/HoBom-s/hobom-event-processor/internal/port/out/poller"
	publisher "github.com/HoBom-s/hobom-event-processor/internal/port/out/publisher"
)

type KafkaPublisher struct {
	Producer *kafka.Producer
}

func NewKafkaPublisher(prod *kafka.Producer) publisher.OutboxEventPublisher {
	return &KafkaPublisher{Producer: prod}
}

func (k *KafkaPublisher) Publish(evt outboxEvent.OutboxEvent) error {
	return k.Producer.Send(evt.EventType, evt.Payload)
}
