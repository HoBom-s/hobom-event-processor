package publisher

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// Event represents a single Kafka message to be published.
type Event struct {
	Key       string
	Value     []byte
	Headers   []kafka.Header
	Timestamp time.Time
	Topic     string
}