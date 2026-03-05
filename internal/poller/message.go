package poller

import "time"

type DeliverHoBomMessageCommand struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Recipient string    `json:"recipient"`
	SenderId  *string   `json:"senderId,omitempty"` // nullable
	SentAt    time.Time `json:"sentAt"`
}

type HoBomSpaceEventCommand struct {
	EntityType string `json:"entityType"`
	Action     string `json:"action"`
	SpaceKey   string `json:"spaceKey"`
	PageId     int64  `json:"pageId"`
	Title      string `json:"title"`
	ActorId    string `json:"actorId"`
}

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
