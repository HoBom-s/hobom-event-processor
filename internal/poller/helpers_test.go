package poller

import (
	"context"
	"errors"
	"testing"

	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
)

// mockPublisher is a test double for publisher.KafkaPublisher.
type mockPublisher struct {
	callCount int
	failUntil int // fail the first N calls
	failErr   error
}

func (m *mockPublisher) Publish(_ context.Context, _ publisher.Event) error {
	m.callCount++
	if m.callCount <= m.failUntil {
		return m.failErr
	}
	return nil
}

func (m *mockPublisher) Close() error { return nil }

func TestPublishWithRetry_SuccessFirstAttempt(t *testing.T) {
	pub := &mockPublisher{}
	err := publishWithRetry(context.Background(), pub, publisher.Event{Topic: "test"})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if pub.callCount != 1 {
		t.Errorf("expected 1 call, got %d", pub.callCount)
	}
}

func TestPublishWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	pub := &mockPublisher{failUntil: 1, failErr: errors.New("transient error")}
	err := publishWithRetry(context.Background(), pub, publisher.Event{Topic: "test"})
	if err != nil {
		t.Fatalf("expected success on retry, got %v", err)
	}
	if pub.callCount != 2 {
		t.Errorf("expected 2 calls, got %d", pub.callCount)
	}
}

func TestPublishWithRetry_AllAttemptsFail(t *testing.T) {
	wantErr := errors.New("persistent kafka error")
	pub := &mockPublisher{failUntil: 99, failErr: wantErr}

	err := publishWithRetry(context.Background(), pub, publisher.Event{Topic: "test"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if pub.callCount != 3 {
		t.Errorf("expected exactly 3 attempts (maxAttempts), got %d", pub.callCount)
	}
}

func TestPublishWithRetry_ContextCancelledBetweenRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before first call

	pub := &mockPublisher{failUntil: 99, failErr: errors.New("err")}
	err := publishWithRetry(ctx, pub, publisher.Event{Topic: "test"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// First publish attempt is always made; context is checked between retries.
	if pub.callCount != 1 {
		t.Errorf("expected 1 call before context cancel, got %d", pub.callCount)
	}
}
