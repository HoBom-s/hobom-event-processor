package port

type OutboxEvent struct {
	ID 				string
	ServiceType 	string
	EventType 		string
	Payload 		[]byte
	Timestamp 		int64
}

type OutboxPoller interface {
	Poll() ([]OutboxEvent, error)
	Ack(eventID string) error
}