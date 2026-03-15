package poller

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped error to contain %v, got %v", wantErr, err)
	}
	if pub.callCount != 3 {
		t.Errorf("expected exactly 3 attempts (maxAttempts), got %d", pub.callCount)
	}
}

func TestPublishWithRetry_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pub := &mockPublisher{failUntil: 99, failErr: errors.New("err")}
	err := publishWithRetry(ctx, pub, publisher.Event{Topic: "test"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if pub.callCount != 0 {
		t.Errorf("expected 0 calls when context is pre-cancelled, got %d", pub.callCount)
	}
}

// --- retryWithBackoff ---

func TestRetryWithBackoff_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := retryWithBackoff(context.Background(), 3, time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_SuccessOnLastAttempt(t *testing.T) {
	calls := 0
	err := retryWithBackoff(context.Background(), 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_AllFail(t *testing.T) {
	underlying := errors.New("always fails")
	calls := 0
	err := retryWithBackoff(context.Background(), 2, time.Millisecond, func() error {
		calls++
		return underlying
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, underlying) {
		t.Errorf("expected wrapped error to contain %v, got %v", underlying, err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := retryWithBackoff(ctx, 3, time.Millisecond, func() error {
		calls++
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls with pre-cancelled context, got %d", calls)
	}
}
