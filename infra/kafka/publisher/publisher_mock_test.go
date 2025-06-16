package publisher

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type mockKafkaWriter struct {
	WriteFunc func(ctx context.Context, msgs ...kafka.Message) error
	CloseFunc func() error
}

func (m *mockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(ctx, msgs...)
	}
	return nil
}

func (m *mockKafkaWriter) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
