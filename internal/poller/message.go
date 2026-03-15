package poller

import "time"

// DeliverHoBomMessageCommand is the Kafka payload for user-to-user messages.
// Published to "hobom.messages" topic. Downstream consumers use the Type field
// to route delivery (MAIL_MESSAGE → email, PUSH_MESSAGE → push notification).
type DeliverHoBomMessageCommand struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Recipient string    `json:"recipient"`
	SenderId  *string   `json:"senderId,omitempty"`
	SentAt    time.Time `json:"sentAt"`
}

// HoBomSpaceEventCommand is the Kafka payload for space document events.
// Published to "hobom.space-events" topic. Represents CRUD operations on
// spaces, pages, and comments (e.g. page created, comment added).
type HoBomSpaceEventCommand struct {
	EntityType string `json:"entityType"`
	Action     string `json:"action"`
	SpaceKey   string `json:"spaceKey"`
	PageId     int64  `json:"pageId"`
	Title      string `json:"title"`
	ActorId    string `json:"actorId"`
}

// HoBomLogMessageCommand is the Kafka payload for API request/response logs.
// Published to "hobom.logs" topic. Both LogPoller and SpaceLogPoller produce
// this type — LogPoller from for-hobom-backend, SpaceLogPoller from
// hobom-space-backend. They are batched as JSON arrays for efficiency.
type HoBomLogMessageCommand struct {
	ServiceType string                 `json:"serviceType"`
	Level       string                 `json:"level"`
	TraceId     string                 `json:"traceId"`
	Message     string                 `json:"message"`
	HttpMethod  string                 `json:"httpMethod"`
	Path        *string                `json:"path,omitempty"`
	StatusCode  int                    `json:"statusCode"`
	Host        string                 `json:"host"`
	UserId      string                 `json:"userId"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}
