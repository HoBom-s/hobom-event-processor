package publisher

import (
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaConfig struct {
	Brokers      []string
	Timeout      time.Duration
	Acks         kafka.RequiredAcks
	Balancer     kafka.Balancer
}
