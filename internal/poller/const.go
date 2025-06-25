package poller

const (
  // Define Kafka Event Type
  EventTypeTodayMenu 	= "TODAY_MENU"

  // Outbox Status
  OutboxPending 		= "PENDING"
  OutboxSent    		= "SENT"
  OutboxFailed			= "FAILED"
)