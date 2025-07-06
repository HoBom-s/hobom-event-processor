package poller

const (
  // Define Kafka Event Types
  EventTypeTodayMenu 	= "TODAY_MENU"
  EventTypeHoBomLog   = "HOBOM_LOG"

  // Outbox Statuss
  OutboxPending 		= "PENDING"
  OutboxSent    		= "SENT"
  OutboxFailed			= "FAILED"

  // Kafka Topics
  HoBomMessage     = "hobom.messages"
  HoBomLog         = "hobom.logs"

  // Message Types
  Mail            = "MAIL_MESSAGE"
  Push            = "PUSH_MESSAGE"

  // DLQ Keys
  HoBomTodayMenuDLQPrefix    = "dlq:menu:"
  HoBomLogDLQPrefix          = "dlq:log:"
)