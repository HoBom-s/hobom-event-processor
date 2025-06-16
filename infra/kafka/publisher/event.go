package publisher

import (
	"time"

	"github.com/segmentio/kafka-go"
)

type Event struct {
	Key				string
	Value			[]byte
	Headers			[]kafka.Header
	Timestamp		time.Time
	Topic			string
}