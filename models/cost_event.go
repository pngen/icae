package models

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// CostEvent represents a single unit of cost attribution in the system.
type CostEvent struct {
	EventID        string                 `json:"event_id"`
	Timestamp      time.Time              `json:"timestamp"`
	ExecutionID    string                 `json:"execution_id"`
	Component      string                 `json:"component"`
	Action         string                 `json:"action"`
	UnitCost       float64                `json:"unit_cost"`
	Quantity       float64                `json:"quantity"`
	TotalCost      float64                `json:"total_cost"`
	Currency       string                 `json:"currency"`
	CostSource     string                 `json:"cost_source"`
	PricingVersion string                 `json:"pricing_version"`
	BaseUnit       string                 `json:"base_unit"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Validate performs comprehensive validation of the CostEvent.
// Returns nil if valid, or a ValidationError describing the issue.
func (e *CostEvent) Validate() error {
	var errs []string

	// Required field validation
	if e.EventID == "" {
		errs = append(errs, "event_id is required")
	}
	if e.ExecutionID == "" {
		errs = append(errs, "execution_id is required")
	}
	if e.Component == "" {
		errs = append(errs, "component is required")
	}
	if e.Action == "" {
		errs = append(errs, "action is required")
	}
	if e.Currency == "" {
		errs = append(errs, "currency is required")
	}
	if e.CostSource == "" {
		errs = append(errs, "cost_source is required")
	}
	if e.PricingVersion == "" {
		errs = append(errs, "pricing_version is required")
	}
	if e.BaseUnit == "" {
		errs = append(errs, "base_unit is required")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "timestamp is required")
	}

	// Numeric validation
	if !isFinite(e.UnitCost) {
		errs = append(errs, "unit_cost must be finite")
	} else if e.UnitCost < 0 {
		errs = append(errs, "unit_cost cannot be negative")
	}
	if !isFinite(e.Quantity) {
		errs = append(errs, "quantity must be finite")
	} else if e.Quantity < 0 {
		errs = append(errs, "quantity cannot be negative")
	}
	if !isFinite(e.TotalCost) {
		errs = append(errs, "total_cost must be finite")
	} else if e.TotalCost < 0 {
		errs = append(errs, "total_cost cannot be negative")
	}

	// Cost consistency validation with epsilon for floating-point comparison
	const epsilon = 1e-9
	expectedCost := e.UnitCost * e.Quantity
	if isFinite(e.UnitCost) && isFinite(e.Quantity) && isFinite(e.TotalCost) && math.Abs(e.TotalCost-expectedCost) > epsilon {
		errs = append(errs, fmt.Sprintf(
			"total_cost (%.9f) must equal unit_cost * quantity (%.9f * %.9f = %.9f)",
			e.TotalCost, e.UnitCost, e.Quantity, expectedCost))
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidationError represents an error during validation.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("validation error: %s", e.Errors[0])
	}
	return fmt.Sprintf("validation errors: %v", e.Errors)
}

// Unwrap returns nil as ValidationError is a leaf error.
func (e *ValidationError) Unwrap() error {
	return nil
}

// Is implements error matching for ValidationError.
func (e *ValidationError) Is(target error) bool {
	_, ok := target.(*ValidationError)
	return ok
}

// ErrValidation is a sentinel error for validation failures.
var ErrValidation = errors.New("validation error")

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
