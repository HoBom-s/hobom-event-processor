package publisher

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaConfig holds configuration for the Kafka publisher.
type KafkaConfig struct {
	Brokers  []string
	Timeout  time.Duration
	Acks     kafka.RequiredAcks
	Balancer kafka.Balancer
}

// DefaultKafkaConfig returns a KafkaConfig with production-safe defaults:
//   - Timeout: 10s
//   - Acks: RequireOne (leader acknowledgement)
//   - Balancer: LeastBytes
func DefaultKafkaConfig(brokers []string) KafkaConfig {
	return KafkaConfig{
		Brokers:  brokers,
		Timeout:  10 * time.Second,
		Acks:     kafka.RequireOne,
		Balancer: &kafka.LeastBytes{},
	}
}
