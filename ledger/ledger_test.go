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

func TestAddEvent_DuplicateEventID(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	e1 := createValidEvent("evt-001", now)
	e2 := createValidEvent("evt-001", now.Add(time.Second))

	if err := l.AddEvent(e1); err != nil {
		t.Fatalf("first AddEvent failed: %v", err)
	}

	err := l.AddEvent(e2)
	if err == nil {
		t.Fatal("expected duplicate event_id error, got nil")
	}
	if !errors.Is(err, ErrDuplicateEventID) {
		t.Errorf("expected ErrDuplicateEventID, got %v", err)
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

func TestEventMetadataCannotMutateLedger(t *testing.T) {
	l := NewCostLedger()
	event := createValidEvent("evt-001", time.Now().UTC())
	event.Metadata = map[string]interface{}{"source": "initial"}

	if err := l.AddEvent(event); err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}
	originalHash := l.GetLedgerHash()

	event.Metadata["source"] = "mutated after add"
	events := l.GetEvents()
	events[0].Metadata["source"] = "mutated from copy"

	stored := l.GetEvents()
	if stored[0].Metadata["source"] != "initial" {
		t.Fatalf("ledger metadata was externally mutated: %v", stored[0].Metadata["source"])
	}
	if l.GetLedgerHash() != originalHash {
		t.Fatal("ledger hash changed after external metadata mutation")
	}
}

func TestNestedEventMetadataCannotMutateLedger(t *testing.T) {
	type details struct {
		Labels []string `json:"labels"`
	}

	l := NewCostLedger()
	event := createValidEvent("evt-001", time.Now().UTC())
	nestedMap := map[string]interface{}{
		"values": []interface{}{map[string]interface{}{"value": "initial"}},
	}
	typedDetails := &details{Labels: []string{"initial"}}
	event.Metadata = map[string]interface{}{
		"nested":  nestedMap,
		"details": typedDetails,
	}

	if err := l.AddEvent(event); err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}
	originalHash := l.GetLedgerHash()

	nestedMap["values"].([]interface{})[0].(map[string]interface{})["value"] = "mutated after add"
	typedDetails.Labels[0] = "mutated after add"

	readCopy := l.GetEvents()
	readNested := readCopy[0].Metadata["nested"].(map[string]interface{})
	readNested["values"].([]interface{})[0].(map[string]interface{})["value"] = "mutated from copy"
	readDetails, ok := readCopy[0].Metadata["details"].(*details)
	if !ok {
		t.Fatalf("metadata type was not preserved: %T", readCopy[0].Metadata["details"])
	}
	readDetails.Labels[0] = "mutated from copy"

	stored := l.GetEvents()[0].Metadata
	storedNested := stored["nested"].(map[string]interface{})
	if got := storedNested["values"].([]interface{})[0].(map[string]interface{})["value"]; got != "initial" {
		t.Fatalf("nested ledger metadata was externally mutated: %v", got)
	}
	storedDetails := stored["details"].(*details)
	if got := storedDetails.Labels[0]; got != "initial" {
		t.Fatalf("typed ledger metadata was externally mutated: %v", got)
	}
	if l.GetLedgerHash() != originalHash {
		t.Fatal("ledger hash changed after external nested metadata mutation")
	}
}

func TestLedgerHashPreservesExactFloatValues(t *testing.T) {
	now := time.Now().UTC()

	zeroLedger := NewCostLedger()
	zeroEvent := createValidEvent("evt-001", now)
	zeroEvent.UnitCost = 0
	zeroEvent.Quantity = 1
	zeroEvent.TotalCost = 0
	if err := zeroLedger.AddEvent(zeroEvent); err != nil {
		t.Fatalf("failed to add zero-cost event: %v", err)
	}

	preciseLedger := NewCostLedger()
	preciseEvent := createValidEvent("evt-001", now)
	preciseEvent.UnitCost = 4e-10
	preciseEvent.Quantity = 1
	preciseEvent.TotalCost = 4e-10
	if err := preciseLedger.AddEvent(preciseEvent); err != nil {
		t.Fatalf("failed to add precise-cost event: %v", err)
	}

	if zeroLedger.GetLedgerHash() == preciseLedger.GetLedgerHash() {
		t.Fatal("distinct accepted float64 costs produced the same ledger hash")
	}
}

func TestLedgerHashCanonicalizesTimestampToUTC(t *testing.T) {
	instant := time.Date(2026, time.January, 2, 3, 4, 5, 6, time.UTC)
	offset := time.FixedZone("test-offset", 5*60*60)

	utcLedger := NewCostLedger()
	if err := utcLedger.AddEvent(createValidEvent("evt-001", instant)); err != nil {
		t.Fatalf("failed to add UTC event: %v", err)
	}

	offsetLedger := NewCostLedger()
	if err := offsetLedger.AddEvent(createValidEvent("evt-001", instant.In(offset))); err != nil {
		t.Fatalf("failed to add offset event: %v", err)
	}

	if utcLedger.GetLedgerHash() != offsetLedger.GetLedgerHash() {
		t.Fatal("the same instant in different locations produced different ledger hashes")
	}
	if got := offsetLedger.GetEvents()[0].Timestamp.Location(); got != time.UTC {
		t.Fatalf("stored timestamp was not canonicalized to UTC: %v", got)
	}
}

func TestAddEvent_RejectsMixedCurrency(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	if err := l.AddEvent(createValidEvent("evt-001", now)); err != nil {
		t.Fatalf("first AddEvent failed: %v", err)
	}
	originalHash := l.GetLedgerHash()

	event := createValidEvent("evt-002", now.Add(time.Second))
	event.Currency = "EUR"
	err := l.AddEvent(event)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
	if l.GetEventCount() != 1 {
		t.Fatalf("mixed-currency event mutated ledger count: %d", l.GetEventCount())
	}
	if l.GetLedgerHash() != originalHash {
		t.Fatal("mixed-currency event mutated ledger hash")
	}
}

func TestAddEvent_EqualTimestampsRequireEventIDOrder(t *testing.T) {
	l := NewCostLedger()
	now := time.Now().UTC()

	if err := l.AddEvent(createValidEvent("evt-002", now)); err != nil {
		t.Fatalf("first AddEvent failed: %v", err)
	}
	err := l.AddEvent(createValidEvent("evt-001", now))
	if !errors.Is(err, ErrChronologicalOrder) {
		t.Fatalf("expected ErrChronologicalOrder for reverse tie order, got %v", err)
	}
	if l.GetEventCount() != 1 {
		t.Fatalf("reverse tie-order event mutated ledger count: %d", l.GetEventCount())
	}

	ordered := NewCostLedger()
	if err := ordered.AddEvent(createValidEvent("evt-001", now)); err != nil {
		t.Fatalf("failed to add first ordered event: %v", err)
	}
	if err := ordered.AddEvent(createValidEvent("evt-002", now)); err != nil {
		t.Fatalf("failed to add second ordered event: %v", err)
	}
	events := ordered.GetEvents()
	if events[0].EventID != "evt-001" || events[1].EventID != "evt-002" {
		t.Fatalf("equal-time events are not in canonical order: %q, %q", events[0].EventID, events[1].EventID)
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
