package repository

import (
	outboxEventLog "github.com/HoBom-s/hobom-event-processor/internal/domain/eventlog"
	outboxEvent "github.com/HoBom-s/hobom-event-processor/internal/port/out/poller"
	"gorm.io/gorm"
)

type EventLogger struct {
	db *gorm.DB
}

func NewEventLogRepository(db *gorm.DB) *EventLogger {
	return &EventLogger{db: db}
}

func (r *EventLogger) Save(event outboxEvent.OutboxEvent) error {
	return r.db.Create(&outboxEventLog.OutboxEventLog{
		ID:        		event.ID,
		ServiceType: 	event.ServiceType,
		EventType: 		event.EventType,
		Payload:   		event.Payload,
		CreatedAt: 		event.Timestamp,
	}).Error
}