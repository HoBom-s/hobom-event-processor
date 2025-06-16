package publisher

import (
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaConfig struct {
	Brokers      []string
	DefaultTopic string
	Timeout      time.Duration
	Acks         kafka.RequiredAcks
	Balancer     kafka.Balancer
}
