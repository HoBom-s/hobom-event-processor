package poller

const (
  // Define Kafka Event Types
  EventTypeTodayMenu 	= "TODAY_MENU"

  // Outbox Statuss
  OutboxPending 		= "PENDING"
  OutboxSent    		= "SENT"
  OutboxFailed			= "FAILED"

  // Kafka Topics
  HoBomMessage     = "hobom.messages"

  // Message Types
  Mail            = "MAIL_MESSAGE"
  Push            = "PUSH_MESSAGE"
)