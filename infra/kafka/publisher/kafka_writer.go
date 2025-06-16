package publisher

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type kafkaWriterImpl struct {
	*kafka.Writer
}

func (w *kafkaWriterImpl) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return w.Writer.WriteMessages(ctx, msgs...)
}

func (w *kafkaWriterImpl) Close() error {
	return w.Writer.Close()
}
