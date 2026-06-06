package models

import (
	"errors"
	"math"
	"testing"
	"time"
)

func createValidCostEvent() CostEvent {
	return CostEvent{
		EventID:        "evt-001",
		Timestamp:      time.Now().UTC(),
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

func TestCostEvent_Validate_Valid(t *testing.T) {
	event := createValidCostEvent()
	if err := event.Validate(); err != nil {
		t.Errorf("expected valid event, got error: %v", err)
	}
}

func TestCostEvent_Validate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CostEvent)
		wantErr string
	}{
		{"missing event_id", func(e *CostEvent) { e.EventID = "" }, "event_id"},
		{"missing execution_id", func(e *CostEvent) { e.ExecutionID = "" }, "execution_id"},
		{"missing component", func(e *CostEvent) { e.Component = "" }, "component"},
		{"missing action", func(e *CostEvent) { e.Action = "" }, "action"},
		{"missing currency", func(e *CostEvent) { e.Currency = "" }, "currency"},
		{"missing cost_source", func(e *CostEvent) { e.CostSource = "" }, "cost_source"},
		{"missing pricing_version", func(e *CostEvent) { e.PricingVersion = "" }, "pricing_version"},
		{"missing base_unit", func(e *CostEvent) { e.BaseUnit = "" }, "base_unit"},
		{"zero timestamp", func(e *CostEvent) { e.Timestamp = time.Time{} }, "timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := createValidCostEvent()
			tt.mutate(&event)
			err := event.Validate()
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			var validErr *ValidationError
			if !errors.As(err, &validErr) {
				t.Fatalf("expected ValidationError, got %T", err)
			}
		})
	}
}

func TestCostEvent_Validate_NegativeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CostEvent)
	}{
		{"negative unit_cost", func(e *CostEvent) {
			e.UnitCost = -0.01
			e.TotalCost = e.UnitCost * e.Quantity
		}},
		{"negative quantity", func(e *CostEvent) {
			e.Quantity = -10
			e.TotalCost = e.UnitCost * e.Quantity
		}},
		{"negative total_cost", func(e *CostEvent) {
			e.TotalCost = -1.0
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := createValidCostEvent()
			tt.mutate(&event)
			err := event.Validate()
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
		})
	}
}

func TestCostEvent_Validate_NonFiniteValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CostEvent)
	}{
		{"nan unit_cost", func(e *CostEvent) { e.UnitCost = math.NaN() }},
		{"infinite quantity", func(e *CostEvent) { e.Quantity = math.Inf(1) }},
		{"infinite total_cost", func(e *CostEvent) { e.TotalCost = math.Inf(1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := createValidCostEvent()
			tt.mutate(&event)
			if err := event.Validate(); err == nil {
				t.Fatal("expected non-finite value to be rejected")
			}
		})
	}
}

func TestCostEvent_Validate_CostMismatch(t *testing.T) {
	event := createValidCostEvent()
	event.TotalCost = 999.99 // Doesn't match UnitCost * Quantity

	err := event.Validate()
	if err == nil {
		t.Fatal("expected error for cost mismatch")
	}

	var validErr *ValidationError
	if !errors.As(err, &validErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestCostEvent_Validate_ZeroQuantity(t *testing.T) {
	event := createValidCostEvent()
	event.Quantity = 0
	event.TotalCost = 0 // 0.03 * 0 = 0

	if err := event.Validate(); err != nil {
		t.Errorf("zero quantity should be valid: %v", err)
	}
}

func TestCostEvent_Validate_FloatingPointPrecision(t *testing.T) {
	event := createValidCostEvent()
	// Use values that might cause floating-point issues
	event.UnitCost = 0.1
	event.Quantity = 3
	event.TotalCost = 0.30000000000000004 // Common floating-point result

	if err := event.Validate(); err != nil {
		t.Errorf("should handle floating-point precision: %v", err)
	}
}

func TestValidationError_Is(t *testing.T) {
	err := &ValidationError{Errors: []string{"test error"}}
	target := &ValidationError{}

	if !errors.Is(err, target) {
		t.Error("ValidationError.Is should match other ValidationErrors")
	}
	if errors.Is(err, errors.New("other")) {
		t.Error("ValidationError.Is should not match other error types")
	}
}
