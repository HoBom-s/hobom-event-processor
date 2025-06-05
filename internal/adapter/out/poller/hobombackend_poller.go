package poller

import (
	port "github.com/HoBom-s/hobom-event-processor/internal/port/out/poller"
)

type HoBomBackendPoller struct {}

func NewHoBomBackendPoller() port.OutboxPoller {
	// @TODO.. process
	return nil
}

func (p *HoBomBackendPoller) Ack(eventID string) error {
	// @TODO after complete..
	return nil
}