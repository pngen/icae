package adapters

import (
	"errors"
	"testing"
	"time"
)

func TestExecutionTranscriptAdapter_Name(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()
	if adapter.Name() != "execution_transcript" {
		t.Errorf("expected 'execution_transcript', got '%s'", adapter.Name())
	}
}

func TestExecutionTranscriptAdapter_ToCostEvents_Valid(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()
	now := time.Now().UTC()

	transcript := &ExecutionTranscript{
		ExecutionID: "exec-001",
		StartTime:   now,
		EndTime:     now.Add(time.Minute),
		Steps: []ExecutionStep{
			{
				StepID:         "step-001",
				Timestamp:      now,
				Component:      "model",
				Action:         "invoke",
				UnitCost:       0.03,
				Quantity:       100,
				Currency:       "USD",
				CostSource:     "openai",
				PricingVersion: "gpt-4:v1.0.0",
				BaseUnit:       "token",
			},
		},
	}

	events, err := adapter.ToCostEvents(transcript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].ExecutionID != "exec-001" {
		t.Errorf("expected execution_id 'exec-001', got '%s'", events[0].ExecutionID)
	}
	if events[0].TotalCost != 3.0 {
		t.Errorf("expected total_cost 3.0, got %f", events[0].TotalCost)
	}
}

func TestExecutionTranscriptAdapter_ToCostEvents_DefaultCurrency(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()
	now := time.Now().UTC()

	transcript := &ExecutionTranscript{
		ExecutionID: "exec-001",
		Steps: []ExecutionStep{
			{
				StepID:         "step-001",
				Timestamp:      now,
				Component:      "model",
				Action:         "invoke",
				UnitCost:       0.03,
				Quantity:       100,
				Currency:       "", // Empty - should use default
				CostSource:     "openai",
				PricingVersion: "gpt-4:v1.0.0",
				BaseUnit:       "token",
			},
		},
	}

	events, err := adapter.ToCostEvents(transcript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if events[0].Currency != "USD" {
		t.Errorf("expected default currency 'USD', got '%s'", events[0].Currency)
	}
}

func TestExecutionTranscriptAdapter_ToCostEvents_WrongType(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()

	_, err := adapter.ToCostEvents("not a transcript")
	if err == nil {
		t.Fatal("expected error for wrong type")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Op != "type_assertion" {
		t.Errorf("expected op 'type_assertion', got '%s'", adapterErr.Op)
	}
}

func TestExecutionTranscriptAdapter_ToCostEvents_NilTranscript(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()

	_, err := adapter.ToCostEvents((*ExecutionTranscript)(nil))
	if err == nil {
		t.Fatal("expected error for nil transcript")
	}
}

func TestExecutionTranscriptAdapter_ToCostEvents_EmptyExecutionID(t *testing.T) {
	adapter := NewExecutionTranscriptAdapter()

	transcript := &ExecutionTranscript{
		ExecutionID: "", // Empty
		Steps:       []ExecutionStep{},
	}

	_, err := adapter.ToCostEvents(transcript)
	if err == nil {
		t.Fatal("expected error for empty execution_id")
	}
}

func TestToolInvocationAdapter_Name(t *testing.T) {
	adapter := NewToolInvocationAdapter()
	if adapter.Name() != "tool_invocation" {
		t.Errorf("expected 'tool_invocation', got '%s'", adapter.Name())
	}
}

func TestAdapterError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	adapterErr := &AdapterError{
		Adapter: "test",
		Op:      "test_op",
		Err:     innerErr,
	}

	if !errors.Is(adapterErr, innerErr) {
		t.Error("AdapterError should unwrap to inner error")
	}
}