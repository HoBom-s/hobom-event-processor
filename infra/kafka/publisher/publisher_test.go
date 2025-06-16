package publisher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaPublisher_Publish_Success(t *testing.T) {
	mockWriter := &mockKafkaWriter{
		WriteFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			if len(msgs) != 1 {
				t.Errorf("expected 1 message, got %d", len(msgs))
			}
			msg := msgs[0]
			if string(msg.Key) != "key1" || string(msg.Value) != "value1" {
				t.Errorf("unexpected message: %v", msg)
			}
			return nil
		},
	}

	p := &kafkaPublisher{
		cfg: KafkaConfig{
			DefaultTopic: "test-topic",
		},
		writer: mockWriter,
	}

	err := p.Publish(context.Background(), Event{
		Key:       "key1",
		Value:     []byte("value1"),
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestKafkaPublisher_Publish_Failure(t *testing.T) {
	mockWriter := &mockKafkaWriter{
		WriteFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			return errors.New("write failed")
		},
	}

	p := &kafkaPublisher{
		cfg: KafkaConfig{
			DefaultTopic: "test-topic",
		},
		writer: mockWriter,
	}

	err := p.Publish(context.Background(), Event{
		Key:       "key2",
		Value:     []byte("value2"),
		Timestamp: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
