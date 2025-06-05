package usecase

import (
	outboxPort "github.com/HoBom-s/hobom-event-processor/internal/port/out/poller"
	eventPublish "github.com/HoBom-s/hobom-event-processor/internal/port/out/publisher"
)

type EventLogger interface {
	Save(event outboxPort.OutboxEvent) error
}

type OutboxProcessor struct {
	Pollers    []outboxPort.OutboxPoller
	Publisher  eventPublish.OutboxEventPublisher
	Repository EventLogger
}

func NewOutboxProcessor(
	poller outboxPort.OutboxPoller,
	publisher eventPublish.OutboxEventPublisher,
	logger EventLogger,
) *OutboxProcessor {
	return &OutboxProcessor{
		Pollers:    []outboxPort.OutboxPoller{poller},
		Publisher:  publisher,
		Repository: logger,
	}
}

func RunOutboxProcessor(proc *OutboxProcessor) {
	go proc.Run()
}

func (p *OutboxProcessor) Run() {
	for _, poller := range p.Pollers {
		events, err := poller.Poll()
		if err != nil {
			continue
		}

		for _, event := range events {
			if err := p.Publisher.Publish(event); err != nil {
				continue
			}
			
			p.Repository.Save(event)
			poller.Ack(event.ID)
		}
	}
}