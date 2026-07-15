package poller

import (
	"testing"
	"time"

	angelPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/angel/outbox/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildAngelEventCommand_Approval(t *testing.T) {
	occurred := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	item := &angelPb.QueryResult{
		EventId:   "evt-1",
		EventType: "ADOPTION_APPROVED",
		Payload: &angelPb.AngelEventPayload{
			Kind: &angelPb.AngelEventPayload_ApprovalApproved{
				ApprovalApproved: &angelPb.ApprovalApproved{
					ApprovalType:    angelPb.ApprovalType_APPROVAL_TYPE_ADOPTION,
					SubjectRef:      "app-1",
					RecipientUserId: "user-1",
					ShelterId:       "shelter-1",
					OccurredAt:      timestamppb.New(occurred),
				},
			},
		},
	}

	cmd := buildAngelEventCommand(item)

	if cmd.EventType != "ADOPTION_APPROVED" {
		t.Fatalf("EventType = %q", cmd.EventType)
	}
	if cmd.ApprovalType != "APPROVAL_TYPE_ADOPTION" {
		t.Fatalf("ApprovalType = %q", cmd.ApprovalType)
	}
	if cmd.RecipientUserId != "user-1" || cmd.SubjectRef != "app-1" || cmd.ShelterId != "shelter-1" {
		t.Fatalf("approval fields not mapped: %+v", cmd)
	}
	if cmd.OccurredAt != "2026-07-15T00:00:00Z" {
		t.Fatalf("OccurredAt = %q", cmd.OccurredAt)
	}
	if cmd.AnimalId != "" || cmd.Reason != "" {
		t.Fatalf("foster fields should be empty for an approval: %+v", cmd)
	}
}

func TestBuildAngelEventCommand_FosterTerminated(t *testing.T) {
	item := &angelPb.QueryResult{
		EventId:   "evt-2",
		EventType: "FOSTER_TERMINATED",
		Payload: &angelPb.AngelEventPayload{
			Kind: &angelPb.AngelEventPayload_FosterTerminated{
				FosterTerminated: &angelPb.FosterTerminated{
					FosterProcessId: "fp-1",
					AnimalId:        "a-1",
					RecipientUserId: "user-2",
					Reason:          angelPb.FosterEndReason_FOSTER_END_REASON_EXPIRED,
				},
			},
		},
	}

	cmd := buildAngelEventCommand(item)

	if cmd.EventType != "FOSTER_TERMINATED" {
		t.Fatalf("EventType = %q", cmd.EventType)
	}
	if cmd.Reason != "FOSTER_END_REASON_EXPIRED" {
		t.Fatalf("Reason = %q", cmd.Reason)
	}
	if cmd.FosterProcessId != "fp-1" || cmd.AnimalId != "a-1" || cmd.RecipientUserId != "user-2" {
		t.Fatalf("foster fields not mapped: %+v", cmd)
	}
	if cmd.ApprovalType != "" || cmd.SubjectRef != "" {
		t.Fatalf("approval fields should be empty for a foster termination: %+v", cmd)
	}
}

func TestBuildAngelEventCommand_NilPayload(t *testing.T) {
	cmd := buildAngelEventCommand(&angelPb.QueryResult{EventId: "evt-3", EventType: "HOBOM_LOG"})
	if cmd.EventType != "HOBOM_LOG" || cmd.RecipientUserId != "" {
		t.Fatalf("unexpected command for nil payload: %+v", cmd)
	}
}
