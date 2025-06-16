package publisher

import (
	"context"

	utils "github.com/HoBom-s/hobom-event-processor/pkg/utils"
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

	topic := event.Topic
	if utils.IsEmptyString(topic) {
		topic = p.cfg.DefaultTopic
	}

	msg := kafka.Message{
		Key:       []byte(event.Key),
		Value:     event.Value,
		Headers:   event.Headers,
		Time:      event.Timestamp,
		Topic:     topic,
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
