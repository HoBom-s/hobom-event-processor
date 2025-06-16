package publisher

import (
	"context"
)

type Hook interface {
	BeforePublish(ctx context.Context, event Event)
	AfterPublish(ctx context.Context, event Event, err error)
}