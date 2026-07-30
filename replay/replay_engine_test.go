package replay

import (
	"errors"
	"math"
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

func TestNewReplayEngine_ClonesPricingModels(t *testing.T) {
	pricingModels := createTestPricingModels()
	model := pricingModels["gpt-4:v1.0.0"]
	model.Metadata = map[string]string{"source": "original"}
	pricingModels["gpt-4:v1.0.0"] = model

	engine := NewReplayEngine(pricingModels)
	model.Tiers[0].UnitCost = 0.99
	model.Metadata["source"] = "mutated"
	delete(pricingModels, "gpt-4:v1.0.0")

	stored, ok := engine.GetPricingModel("gpt-4:v1.0.0")
	if !ok {
		t.Fatal("expected cloned pricing model to remain in engine")
	}
	if stored.Tiers[0].UnitCost != 0.03 {
		t.Fatalf("constructor retained caller tier alias: got %v", stored.Tiers[0].UnitCost)
	}
	if stored.Metadata["source"] != "original" {
		t.Fatalf("constructor retained caller metadata alias: got %q", stored.Metadata["source"])
	}
}

func TestAddPricingModel_ClonesModel(t *testing.T) {
	engine := NewReplayEngine(nil)
	model := createTestPricingModels()["gpt-4:v1.0.0"]
	model.Metadata = map[string]string{"source": "original"}

	if err := engine.AddPricingModel(model); err != nil {
		t.Fatalf("AddPricingModel failed: %v", err)
	}
	model.Tiers[0].UnitCost = 0.99
	model.Metadata["source"] = "mutated"

	stored, ok := engine.GetPricingModel(model.Key())
	if !ok {
		t.Fatal("expected added pricing model")
	}
	if stored.Tiers[0].UnitCost != 0.03 {
		t.Fatalf("AddPricingModel retained caller tier alias: got %v", stored.Tiers[0].UnitCost)
	}
	if stored.Metadata["source"] != "original" {
		t.Fatalf("AddPricingModel retained caller metadata alias: got %q", stored.Metadata["source"])
	}
}

func TestGetPricingModel_ReturnsClone(t *testing.T) {
	pricingModels := createTestPricingModels()
	model := pricingModels["gpt-4:v1.0.0"]
	model.Metadata = map[string]string{"source": "original"}
	pricingModels["gpt-4:v1.0.0"] = model
	engine := NewReplayEngine(pricingModels)

	returned, ok := engine.GetPricingModel("gpt-4:v1.0.0")
	if !ok {
		t.Fatal("expected pricing model")
	}
	returned.Tiers[0].UnitCost = 0.99
	returned.Metadata["source"] = "mutated"

	stored, ok := engine.GetPricingModel("gpt-4:v1.0.0")
	if !ok {
		t.Fatal("expected pricing model on second lookup")
	}
	if stored.Tiers[0].UnitCost != 0.03 {
		t.Fatalf("GetPricingModel exposed internal tier alias: got %v", stored.Tiers[0].UnitCost)
	}
	if stored.Metadata["source"] != "original" {
		t.Fatalf("GetPricingModel exposed internal metadata alias: got %q", stored.Metadata["source"])
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

func TestReplayExecution_BaseUnitMismatch(t *testing.T) {
	engine := NewReplayEngine(createTestPricingModels())
	l := ledger.NewCostLedger()
	event := createTestEvent("evt-001", time.Now().UTC(), "exec-001")
	event.BaseUnit = "request"

	if err := l.AddEvent(event); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}
	_, err := engine.ReplayExecution("exec-001", l)
	if !errors.Is(err, ErrBaseUnitMismatch) {
		t.Fatalf("expected ErrBaseUnitMismatch, got %v", err)
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

func TestCompareReplayWithOriginal_Mismatch(t *testing.T) {
	pricingModels := createTestPricingModels()
	model := pricingModels["gpt-4:v1.0.0"]
	model.Tiers[0].UnitCost = 0.04
	pricingModels["gpt-4:v1.0.0"] = model
	engine := NewReplayEngine(pricingModels)
	l := ledger.NewCostLedger()
	event := createTestEvent("evt-001", time.Now().UTC(), "exec-001")

	if err := l.AddEvent(event); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}
	result := engine.CompareReplayWithOriginal("exec-001", l)

	if result.Status != "mismatch" {
		t.Fatalf("expected mismatch status, got %q (%s)", result.Status, result.Error)
	}
	if result.Error != "" {
		t.Fatalf("expected ordinary drift not to be reported as an error, got %q", result.Error)
	}
	if math.Abs(result.OriginalCost-3.0) > replayEpsilon {
		t.Fatalf("expected original cost 3.0, got %v", result.OriginalCost)
	}
	if math.Abs(result.ReplayedCost-4.0) > replayEpsilon {
		t.Fatalf("expected replayed cost 4.0, got %v", result.ReplayedCost)
	}
	if math.Abs(result.Delta-1.0) > replayEpsilon {
		t.Fatalf("expected delta 1.0, got %v", result.Delta)
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
