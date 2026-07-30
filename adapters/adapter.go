package adapters

import (
	"fmt"
	"time"

	"icae/models"
)

// CostAdapter defines the interface for converting external data into cost events.
// Implementations must be thread-safe if used concurrently.
type CostAdapter interface {
	// ToCostEvents converts external data into a slice of CostEvents.
	// Returns an error if the data cannot be converted.
	ToCostEvents(data interface{}) ([]models.CostEvent, error)

	// Name returns the adapter's identifier for logging and debugging.
	Name() string
}

// AdapterError wraps errors from adapter operations with context.
type AdapterError struct {
	Adapter string
	Op      string
	Err     error
}

func (e *AdapterError) Error() string {
	return fmt.Sprintf("adapter %s: %s: %v", e.Adapter, e.Op, e.Err)
}

func (e *AdapterError) Unwrap() error {
	return e.Err
}

// ExecutionTranscript represents the input format for execution transcript data.
type ExecutionTranscript struct {
	ExecutionID string                 `json:"execution_id"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Steps       []ExecutionStep        `json:"steps"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionStep represents a single step in an execution transcript.
type ExecutionStep struct {
	StepID         string                 `json:"step_id"`
	Timestamp      time.Time              `json:"timestamp"`
	Component      string                 `json:"component"`
	Action         string                 `json:"action"`
	UnitCost       float64                `json:"unit_cost"`
	FixedFee       float64                `json:"fixed_fee,omitempty"`
	Quantity       float64                `json:"quantity"`
	Currency       string                 `json:"currency"`
	CostSource     string                 `json:"cost_source"`
	PricingVersion string                 `json:"pricing_version"`
	BaseUnit       string                 `json:"base_unit"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionTranscriptAdapter converts execution transcripts into cost events.
type ExecutionTranscriptAdapter struct {
	// DefaultCurrency is used when a step doesn't specify currency.
	DefaultCurrency string
}

// NewExecutionTranscriptAdapter creates a new adapter with sensible defaults.
func NewExecutionTranscriptAdapter() *ExecutionTranscriptAdapter {
	return &ExecutionTranscriptAdapter{
		DefaultCurrency: "USD",
	}
}

// Name returns the adapter identifier.
func (a *ExecutionTranscriptAdapter) Name() string {
	return "execution_transcript"
}

// ToCostEvents converts execution transcript data into a list of cost events.
func (a *ExecutionTranscriptAdapter) ToCostEvents(data interface{}) ([]models.CostEvent, error) {
	transcript, ok := data.(*ExecutionTranscript)
	if !ok {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "type_assertion",
			Err:     fmt.Errorf("expected *ExecutionTranscript, got %T", data),
		}
	}

	if transcript == nil {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "validate",
			Err:     fmt.Errorf("transcript cannot be nil"),
		}
	}

	if transcript.ExecutionID == "" {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "validate",
			Err:     fmt.Errorf("execution_id is required"),
		}
	}

	events := make([]models.CostEvent, 0, len(transcript.Steps))

	for i, step := range transcript.Steps {
		currency := step.Currency
		if currency == "" {
			currency = a.DefaultCurrency
		}

		event := models.CostEvent{
			EventID:        step.StepID,
			Timestamp:      step.Timestamp,
			ExecutionID:    transcript.ExecutionID,
			Component:      step.Component,
			Action:         step.Action,
			UnitCost:       step.UnitCost,
			FixedFee:       step.FixedFee,
			Quantity:       step.Quantity,
			TotalCost:      step.FixedFee + step.UnitCost*step.Quantity,
			Currency:       currency,
			CostSource:     step.CostSource,
			PricingVersion: step.PricingVersion,
			BaseUnit:       step.BaseUnit,
			Metadata:       step.Metadata,
		}

		if err := event.Validate(); err != nil {
			return nil, &AdapterError{
				Adapter: a.Name(),
				Op:      fmt.Sprintf("validate_step[%d]", i),
				Err:     err,
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// ToolInvocationAdapter converts tool invocation logs into cost events.
type ToolInvocationAdapter struct {
	DefaultCurrency string
}

// NewToolInvocationAdapter creates a new adapter with sensible defaults.
func NewToolInvocationAdapter() *ToolInvocationAdapter {
	return &ToolInvocationAdapter{
		DefaultCurrency: "USD",
	}
}

// Name returns the adapter identifier.
func (a *ToolInvocationAdapter) Name() string {
	return "tool_invocation"
}

// ToolInvocationLog represents tool invocation data.
type ToolInvocationLog struct {
	ExecutionID string           `json:"execution_id"`
	Invocations []ToolInvocation `json:"invocations"`
}

// ToolInvocation represents a single tool call.
type ToolInvocation struct {
	InvocationID   string                 `json:"invocation_id"`
	Timestamp      time.Time              `json:"timestamp"`
	ToolName       string                 `json:"tool_name"`
	UnitCost       float64                `json:"unit_cost"`
	FixedFee       float64                `json:"fixed_fee,omitempty"`
	Quantity       float64                `json:"quantity"`
	CostSource     string                 `json:"cost_source"`
	PricingVersion string                 `json:"pricing_version"`
	BaseUnit       string                 `json:"base_unit"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ToCostEvents converts tool invocation logs into cost events.
func (a *ToolInvocationAdapter) ToCostEvents(data interface{}) ([]models.CostEvent, error) {
	log, ok := data.(*ToolInvocationLog)
	if !ok {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "type_assertion",
			Err:     fmt.Errorf("expected *ToolInvocationLog, got %T", data),
		}
	}

	if log == nil {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "validate",
			Err:     fmt.Errorf("log cannot be nil"),
		}
	}

	if log.ExecutionID == "" {
		return nil, &AdapterError{
			Adapter: a.Name(),
			Op:      "validate",
			Err:     fmt.Errorf("execution_id is required"),
		}
	}

	events := make([]models.CostEvent, 0, len(log.Invocations))

	for i, inv := range log.Invocations {
		event := models.CostEvent{
			EventID:        inv.InvocationID,
			Timestamp:      inv.Timestamp,
			ExecutionID:    log.ExecutionID,
			Component:      "tool",
			Action:         inv.ToolName,
			UnitCost:       inv.UnitCost,
			FixedFee:       inv.FixedFee,
			Quantity:       inv.Quantity,
			TotalCost:      inv.FixedFee + inv.UnitCost*inv.Quantity,
			Currency:       a.DefaultCurrency,
			CostSource:     inv.CostSource,
			PricingVersion: inv.PricingVersion,
			BaseUnit:       inv.BaseUnit,
			Metadata:       inv.Metadata,
		}

		if err := event.Validate(); err != nil {
			return nil, &AdapterError{
				Adapter: a.Name(),
				Op:      fmt.Sprintf("validate_invocation[%d]", i),
				Err:     err,
			}
		}

		events = append(events, event)
	}

	return events, nil
}
