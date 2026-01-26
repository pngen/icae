package replay

import (
	"errors"
	"testing"
	"time"

	"icae/ledger"
	"icae/models"
)

func createTestPricingModels() map[string]models.PricingModel {
	return map[string]models.PricingModel{
		"gpt-4:v1.0.0": {
			ID:          "pm-001",
			Version:     "v1.0.0",
			Component:   "gpt-4",
			PricingType: "token",
			BaseUnit:    "token",
			Tiers: []models.PricingTier{
				{MinQuantity: 0, MaxQuantity: -1, UnitCost: 0.03},
			},
		},
	}
}

func createTestEvent(id string, ts time.Time, execID string) models.CostEvent {
	return models.CostEvent{
		EventID:        id,
		Timestamp:      ts,
		ExecutionID:    execID,
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

func TestNewReplayEngine(t *testing.T) {
	// Test with nil
	engine := NewReplayEngine(nil)
	if engine == nil {
		t.Fatal("NewReplayEngine returned nil")
	}
	if engine.PricingModels == nil {
		t.Error("PricingModels should not be nil")
	}

	// Test with models
	models := createTestPricingModels()
	engine = NewReplayEngine(models)
	if len(engine.PricingModels) != 1 {
		t.Errorf("expected 1 pricing model, got %d", len(engine.PricingModels))
	}
}

func TestReplayExecution_Success(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())
	l := ledger.NewCostLedger()
	now := time.Now().UTC()

	event := createTestEvent("evt-001", now, "exec-001")
	if err := l.AddEvent(event); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}

	cost, err := engine.ReplayExecution("exec-001", l)
	if err != nil {
		t.Fatalf("ReplayExecution failed: %v", err)
	}

	if cost != 3.0 {
		t.Errorf("expected cost 3.0, got %f", cost)
	}
}

func TestReplayExecution_NilLedger(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())

	_, err := engine.ReplayExecution("exec-001", nil)
	if err == nil {
		t.Fatal("expected error for nil ledger")
	}
	if !errors.Is(err, ErrNilLedger) {
		t.Errorf("expected ErrNilLedger, got %v", err)
	}
}

func TestReplayExecution_NoEvents(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())
	l := ledger.NewCostLedger()

	_, err := engine.ReplayExecution("exec-001", l)
	if err == nil {
		t.Fatal("expected error for no events")
	}
	if !errors.Is(err, ErrNoEventsFound) {
		t.Errorf("expected ErrNoEventsFound, got %v", err)
	}
}

func TestReplayExecution_UnknownPricing(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())
	l := ledger.NewCostLedger()
	now := time.Now().UTC()

	event := createTestEvent("evt-001", now, "exec-001")
	event.PricingVersion = "unknown:v1.0.0"
	// Manually add to bypass validation (for testing)
	l.AddEvent(event)

	_, err := engine.ReplayExecution("exec-001", l)
	if err == nil {
		t.Fatal("expected error for unknown pricing")
	}
	if !errors.Is(err, ErrUnknownPricingVersion) {
		t.Errorf("expected ErrUnknownPricingVersion, got %v", err)
	}
}

func TestCompareReplayWithOriginal(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())
	l := ledger.NewCostLedger()
	now := time.Now().UTC()

	event := createTestEvent("evt-001", now, "exec-001")
	l.AddEvent(event)

	result := engine.CompareReplayWithOriginal("exec-001", l)

	if result.ExecutionID != "exec-001" {
		t.Errorf("expected exec-001, got %s", result.ExecutionID)
	}
	if result.Status != "match" {
		t.Errorf("expected match status, got %s", result.Status)
	}
	if result.OriginalCost != 3.0 {
		t.Errorf("expected original cost 3.0, got %f", result.OriginalCost)
	}
	if result.Delta != 0 {
		t.Errorf("expected delta 0, got %f", result.Delta)
	}
}

func TestCompareReplayWithOriginal_NilLedger(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())

	result := engine.CompareReplayWithOriginal("exec-001", nil)

	if result.Status != "error" {
		t.Errorf("expected error status, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}