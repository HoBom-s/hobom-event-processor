package poller

import "time"

const (
  // Define Kafka Event Types
  EventTypeHoBomMessage 	= "MESSAGE"
  EventTypeHoBomLog       = "HOBOM_LOG"

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
  // DLQ의 Key는 Prefix로 `dlq:`를 가져야 함에 주의하도록 한다.
  HoBomTodayMenuDLQPrefix    = "dlq:menu:"
  HoBomLogDLQPrefix          = "dlq:log:"

  // DLQ TTL
  TTL72Hours                 = 72 * time.Hour

  // Internal API Prefix
  HoBomEventPrcessorInternalApiPrefix     = "/hobom-event-processor/internal/api/v1"
)