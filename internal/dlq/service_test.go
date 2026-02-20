package dlq

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- Test doubles ---

type mockDLQStore struct {
	data map[string][]byte
	err  error
}

func newMockDLQStore() *mockDLQStore {
	return &mockDLQStore{data: make(map[string][]byte)}
}

func (m *mockDLQStore) Save(_ context.Context, key string, payload []byte, _ time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.data[key] = payload
	return nil
}

func (m *mockDLQStore) Get(_ context.Context, key string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.data[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	return v, nil
}

func (m *mockDLQStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

// List returns keys matching the "prefix*" glob pattern used by DLQService.
func (m *mockDLQStore) List(_ context.Context, pattern string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	prefix := strings.TrimSuffix(pattern, "*")
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

type mockKafkaPublisher struct {
	publishErr error
	published  []publisher.Event
}

func (m *mockKafkaPublisher) Publish(_ context.Context, event publisher.Event) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, event)
	return nil
}

func (m *mockKafkaPublisher) Close() error { return nil }

type mockPatchClient struct {
	sentErr    error
	sentCalled bool
}

func (m *mockPatchClient) PatchOutboxMarkAsSentUseCase(_ context.Context, _ *outboxPb.MarkRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.sentCalled = true
	return &emptypb.Empty{}, m.sentErr
}

func (m *mockPatchClient) PatchOutboxMarkAsFailedUseCase(_ context.Context, _ *outboxPb.MarkFailedRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// --- GetDLQS ---

func TestGetDLQS_NoPrefix_ReturnsAllKeys(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:menu:event-1"] = []byte(`{}`)
	store.data["dlq:log:event-2"] = []byte(`{}`)

	svc := NewService(store, &mockKafkaPublisher{}, &mockPatchClient{})
	keys, err := svc.GetDLQS(context.Background(), "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestGetDLQS_WithPrefix_FiltersCorrectly(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:menu:event-1"] = []byte(`{}`)
	store.data["dlq:log:event-2"] = []byte(`{}`)

	svc := NewService(store, &mockKafkaPublisher{}, &mockPatchClient{})
	keys, err := svc.GetDLQS(context.Background(), "dlq:menu:")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "dlq:menu:event-1" {
		t.Errorf("expected only dlq:menu: keys, got %v", keys)
	}
}

func TestGetDLQS_StoreError(t *testing.T) {
	store := newMockDLQStore()
	store.err = errors.New("redis down")

	svc := NewService(store, &mockKafkaPublisher{}, &mockPatchClient{})
	_, err := svc.GetDLQS(context.Background(), "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetDLQValue ---

func TestGetDLQValue_Found(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:menu:event-1"] = []byte(`{"type":"MAIL_MESSAGE"}`)

	svc := NewService(store, &mockKafkaPublisher{}, &mockPatchClient{})
	data, err := svc.GetDLQValue(context.Background(), "dlq:menu:event-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"type":"MAIL_MESSAGE"}` {
		t.Errorf("unexpected data: %s", data)
	}
}

func TestGetDLQValue_NotFound(t *testing.T) {
	svc := NewService(newMockDLQStore(), &mockKafkaPublisher{}, &mockPatchClient{})
	_, err := svc.GetDLQValue(context.Background(), "dlq:menu:nonexistent")

	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

// --- RetryDLQ ---

func TestRetryDLQ_Success(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:menu:event-abc"] = []byte(`{"type":"MAIL_MESSAGE"}`)
	pub := &mockKafkaPublisher{}
	patch := &mockPatchClient{}

	svc := NewService(store, pub, patch)
	err := svc.RetryDLQ(context.Background(), "dlq:menu:event-abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.published))
	}
	if pub.published[0].Topic != "hobom.messages" {
		t.Errorf("expected topic hobom.messages, got %s", pub.published[0].Topic)
	}
	if !patch.sentCalled {
		t.Error("expected PatchOutboxMarkAsSentUseCase to be called")
	}
	if _, exists := store.data["dlq:menu:event-abc"]; exists {
		t.Error("expected DLQ key to be deleted after successful retry")
	}
}

func TestRetryDLQ_PublishError_DLQPreserved(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:menu:event-abc"] = []byte(`{}`)
	pub := &mockKafkaPublisher{publishErr: errors.New("kafka down")}

	svc := NewService(store, pub, &mockPatchClient{})
	err := svc.RetryDLQ(context.Background(), "dlq:menu:event-abc")

	if err == nil {
		t.Fatal("expected error on publish failure, got nil")
	}
	if _, exists := store.data["dlq:menu:event-abc"]; !exists {
		t.Error("DLQ entry must be preserved when publish fails")
	}
}

func TestRetryDLQ_KeyNotFound(t *testing.T) {
	svc := NewService(newMockDLQStore(), &mockKafkaPublisher{}, &mockPatchClient{})
	err := svc.RetryDLQ(context.Background(), "dlq:menu:nonexistent")

	if err == nil {
		t.Fatal("expected error for missing DLQ key, got nil")
	}
}

func TestRetryDLQ_EmptyEventId_ReturnsError(t *testing.T) {
	store := newMockDLQStore()
	// Trailing colon produces an empty event ID after parsing.
	store.data["dlq:menu:"] = []byte(`{}`)

	svc := NewService(store, &mockKafkaPublisher{}, &mockPatchClient{})
	err := svc.RetryDLQ(context.Background(), "dlq:menu:")

	if err == nil {
		t.Fatal("expected error for empty event ID, got nil")
	}
}

func TestRetryDLQ_CorrectTopicForLogKey(t *testing.T) {
	store := newMockDLQStore()
	store.data["dlq:log:event-xyz"] = []byte(`[{"level":"INFO"}]`)
	pub := &mockKafkaPublisher{}

	svc := NewService(store, pub, &mockPatchClient{})
	err := svc.RetryDLQ(context.Background(), "dlq:log:event-xyz")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.published[0].Topic != "hobom.logs" {
		t.Errorf("expected topic hobom.logs, got %s", pub.published[0].Topic)
	}
}
