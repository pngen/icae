package ledger

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"icae/models"
)

func createValidEvent(id string, ts time.Time) models.CostEvent {
	return models.CostEvent{
		EventID:        id,
		Timestamp:      ts,
		ExecutionID:    "exec-001",
		Component:      "model",
		Action:         "invoke",
		UnitCost:       0.03,
		Quantity:       100,
		TotalCost:      3.0,
		Currency:       "USD",
		CostSource:     "openai",
		PricingVersion: "gpt-4:v1.0.0",
		BaseUnit:       "token",
	}
}

func TestNewCostLedger(t *testing.T) {
	l := NewCostLedger()
	if l == nil {
		t.Fatal("NewCostLedger returned nil")
	}
	if len(l.events) != 0 {
		t.Errorf("expected 0 events, got %d", len(l.events))
	}
	if len(l.hashes) != 0 {
		t.Errorf("expected 0 hashes, got %d", len(l.hashes))
	}
}

func TestAddEvent_Valid(t *testing.T) {
	l := NewCostLedger()
	event := createValidEvent("evt-001", time.Now().UTC())

	err := l.AddEvent(event)
	if err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}

	if l.GetEventCount() != 1 {
		t.Errorf("expected 1 event, got %d", l.GetEventCount())
	}
}

func TestAddEvent_ValidationFailure(t *testing.T) {
	l := NewCostLedger()
	event := models.CostEvent{} // Empty event should fail validation

	err := l.AddEvent(event)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !errors.Is(err, ErrEventValidation) {
		t.Errorf("expected ErrEventValidation, got %v", err)
	}
}

func TestAddEvent_ChronologicalOrder(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	e1 := createValidEvent("evt-001", now)
	e2 := createValidEvent("evt-002", now.Add(-time.Hour)) // Before e1

	if err := l.AddEvent(e1); err != nil {
		t.Fatalf("first AddEvent failed: %v", err)
	}

	err := l.AddEvent(e2)
	if err == nil {
		t.Fatal("expected chronological order error, got nil")
	}
	if !errors.Is(err, ErrChronologicalOrder) {
		t.Errorf("expected ErrChronologicalOrder, got %v", err)
	}
}

func TestGetEvents_ReturnsCopy(t *testing.T) {
	l := NewCostLedger()
	event := createValidEvent("evt-001", time.Now().UTC())
	l.AddEvent(event)

	events := l.GetEvents()
	events[0].TotalCost = 999.99 // Modify returned slice

	// Original should be unchanged
	original := l.GetEvents()
	if original[0].TotalCost == 999.99 {
		t.Error("GetEvents did not return a copy - original was modified")
	}
}

func TestGetLedgerHash_Empty(t *testing.T) {
	l := NewCostLedger()
	hash := l.GetLedgerHash()
	if hash == "" {
		t.Error("empty ledger should have a valid hash")
	}
}

func TestGetLedgerHash_Deterministic(t *testing.T) {
	now := time.Now().UTC()

	l1 := NewCostLedger()
	l1.AddEvent(createValidEvent("evt-001", now))

	l2 := NewCostLedger()
	l2.AddEvent(createValidEvent("evt-001", now))

	if l1.GetLedgerHash() != l2.GetLedgerHash() {
		t.Error("identical ledgers should have identical hashes")
	}
}

func TestVerifyIntegrity(t *testing.T) {
	l := NewCostLedger()
	l.AddEvent(createValidEvent("evt-001", time.Now().UTC()))

	hash := l.GetLedgerHash()

	if err := l.VerifyIntegrity(hash); err != nil {
		t.Errorf("integrity check should pass: %v", err)
	}

	if err := l.VerifyIntegrity("invalid-hash"); err == nil {
		t.Error("integrity check should fail for wrong hash")
	}
}

func TestGetTotalCost(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	e1 := createValidEvent("evt-001", now)
	e1.TotalCost = 10.0
	e1.UnitCost = 0.1
	e1.Quantity = 100

	e2 := createValidEvent("evt-002", now.Add(time.Second))
	e2.TotalCost = 5.0
	e2.UnitCost = 0.05
	e2.Quantity = 100

	l.AddEvent(e1)
	l.AddEvent(e2)

	if l.GetTotalCost() != 15.0 {
		t.Errorf("expected total cost 15.0, got %f", l.GetTotalCost())
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := NewCostLedger()
	baseTime := time.Now().UTC()

	var wg sync.WaitGroup

	// Sequential writes first (to satisfy ordering constraint)
	for i := 0; i < 10; i++ {
		event := createValidEvent(
			fmt.Sprintf("evt-%03d", i),
			baseTime.Add(time.Duration(i)*time.Second),
		)
		l.AddEvent(event)
	}

	// Concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.GetEvents()
			_ = l.GetLedgerHash()
			_ = l.GetTotalCost()
			_ = l.GetEventCount()
		}()
	}

	wg.Wait()
	// No data race panics means success
}

func TestGetEventsByExecution(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	e1 := createValidEvent("evt-001", now)
	e1.ExecutionID = "exec-A"

	e2 := createValidEvent("evt-002", now.Add(time.Second))
	e2.ExecutionID = "exec-B"

	e3 := createValidEvent("evt-003", now.Add(2*time.Second))
	e3.ExecutionID = "exec-A"

	l.AddEvent(e1)
	l.AddEvent(e2)
	l.AddEvent(e3)

	execA := l.GetEventsByExecution("exec-A")
	if len(execA) != 2 {
		t.Errorf("expected 2 events for exec-A, got %d", len(execA))
	}

	execB := l.GetEventsByExecution("exec-B")
	if len(execB) != 1 {
		t.Errorf("expected 1 event for exec-B, got %d", len(execB))
	}

	execC := l.GetEventsByExecution("exec-C")
	if len(execC) != 0 {
		t.Errorf("expected 0 events for exec-C, got %d", len(execC))
	}
}

func TestSnapshot(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	e1 := createValidEvent("evt-001", now)
	l.AddEvent(e1)

	// This should not deadlock
	snapshot := l.Snapshot()

	if snapshot.EventCount != 1 {
		t.Errorf("expected 1 event in snapshot, got %d", snapshot.EventCount)
	}
	if len(snapshot.Events) != 1 {
		t.Errorf("expected 1 event in snapshot.Events, got %d", len(snapshot.Events))
	}
	if snapshot.CurrentHash == "" {
		t.Error("snapshot should have a hash")
	}
	if snapshot.CurrentHash != l.GetLedgerHash() {
		t.Error("snapshot hash should match ledger hash")
	}
}