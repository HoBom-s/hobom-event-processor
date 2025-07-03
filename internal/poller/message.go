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
