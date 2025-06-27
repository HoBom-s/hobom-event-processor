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