package kafka

import (
	"fmt"
)

type Producer struct{}

func NewKafkaProducer() *Producer {
	return &Producer{}
}

func (p *Producer) Send(topic string, payload []byte) error {
	fmt.Printf("Sending to Kafka topic %s: %s\n", topic, string(payload))
	return nil
}
