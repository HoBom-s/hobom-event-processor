package publisher

import (
	"context"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher is the port for publishing events to Kafka.
// Implementations must be safe for concurrent use.
type KafkaPublisher interface {
	// Publish sends a single event to the topic specified in event.Topic.
	Publish(ctx context.Context, event Event) error
	// Close flushes pending messages and closes the underlying writer.
	Close() error
}

type kafkaPublisher struct {
	cfg    		KafkaConfig
	writer 		kafkaWriter
	hooks  		[]Hook
}

func NewKafkaPublisher(cfg KafkaConfig, hooks ...Hook) KafkaPublisher {
	writer := &kafkaWriterImpl{
		Writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Balancer:     cfg.Balancer,
			WriteTimeout: cfg.Timeout,
			RequiredAcks: cfg.Acks,
		},
	}
	slog.Info("kafka publisher created", "brokers", cfg.Brokers)

	return &kafkaPublisher{
		cfg:    cfg,
		writer: writer,
		hooks:  hooks,
	}
}

func (p *kafkaPublisher) Publish(ctx context.Context, event Event) error {
	for _, hook := range p.hooks {
		hook.BeforePublish(ctx, event)
	}


	msg := kafka.Message{
		Key:       []byte(event.Key),
		Value:     event.Value,
		Headers:   event.Headers,
		Time:      event.Timestamp,
		Topic:     event.Topic,
	}

	err := p.writer.WriteMessages(ctx, msg)

	for _, hook := range p.hooks {
		hook.AfterPublish(ctx, event, err)
	}

	return err
}

func (p *kafkaPublisher) Close() error {
	return p.writer.Close()
}
