package eventlog

type OutboxEventLog struct {
	ID        		string `gorm:"primaryKey"`
	ServiceType 	string
	EventType 		string
	Payload   		[]byte
	CreatedAt 		int64
}