package publisher

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type KafkaPublisher interface {
	Publish(ctx context.Context, event Event) error
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
			Topic:        cfg.DefaultTopic,
			Balancer:     cfg.Balancer,
			WriteTimeout: cfg.Timeout,
			RequiredAcks: cfg.Acks,
		},
	}
	fmt.Println("🎃 Kafka connected")

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
