package replay

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"icae/ledger"
	"icae/models"
)

// replayEpsilon is used for floating-point comparisons in cost verification.
const replayEpsilon = 1e-6

// Sentinel errors for replay operations.
var (
	ErrUnknownPricingVersion = errors.New("unknown pricing version")
	ErrCostMismatch          = errors.New("replayed cost does not match recorded cost")
	ErrBaseUnitMismatch      = errors.New("pricing model base unit does not match event base unit")
	ErrNoEventsFound         = errors.New("no events found for execution")
	ErrNilLedger             = errors.New("ledger cannot be nil")
)

// ReplayEngine supports deterministic cost recomputation and delta analysis.
type ReplayEngine struct {
	mu            sync.RWMutex
	PricingModels map[string]models.PricingModel
}

// A nil pricingModels map is treated as an empty map.
func NewReplayEngine(pricingModels map[string]models.PricingModel) *ReplayEngine {
	return &ReplayEngine{
		PricingModels: clonePricingModels(pricingModels),
	}
}

// AddPricingModel adds or updates a pricing model in the engine.
func (r *ReplayEngine) AddPricingModel(model models.PricingModel) error {
	if err := model.Validate(); err != nil {
		return fmt.Errorf("invalid pricing model: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.PricingModels[model.Key()] = clonePricingModel(model)
	return nil
}

// GetPricingModel retrieves a pricing model by its key.
func (r *ReplayEngine) GetPricingModel(key string) (models.PricingModel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.PricingModels[key]
	if !exists {
		return models.PricingModel{}, false
	}
	return clonePricingModel(model), true
}

// ReplayExecution recomputes the total cost for an execution using current pricing models.
// Returns the total replayed cost and any error encountered.
func (r *ReplayEngine) ReplayExecution(executionID string, l *ledger.CostLedger) (float64, error) {
	return r.replayExecution(executionID, l, true)
}

// replayExecution recomputes an execution and optionally verifies every recalculated
// event cost against its recorded value. Comparison callers disable verification so
// ordinary pricing drift can be returned as a structured mismatch and delta.
func (r *ReplayEngine) replayExecution(executionID string, l *ledger.CostLedger, verifyRecordedCost bool) (float64, error) {
	if l == nil {
		return 0, ErrNilLedger
	}

	events := l.GetEventsByExecution(executionID)
	if len(events) == 0 {
		return 0, fmt.Errorf("%w: execution_id=%s", ErrNoEventsFound, executionID)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	totalCost := 0.0

	for _, event := range events {
		// Validate that we have the correct pricing model
		pricingModel, exists := r.PricingModels[event.PricingVersion]
		if !exists {
			return 0, fmt.Errorf("%w: %s", ErrUnknownPricingVersion, event.PricingVersion)
		}
		if pricingModel.BaseUnit != event.BaseUnit {
			return 0, fmt.Errorf("%w: event %s uses %q, pricing model %s uses %q",
				ErrBaseUnitMismatch, event.EventID, event.BaseUnit, event.PricingVersion, pricingModel.BaseUnit)
		}

		// Recalculate cost using current pricing
		calculatedCost, err := pricingModel.CalculateCost(event.Quantity)
		if err != nil {
			return 0, fmt.Errorf("failed to calculate cost for event %s: %w", event.EventID, err)
		}

		// Verify that the event's cost matches our calculation
		if verifyRecordedCost && math.Abs(calculatedCost-event.TotalCost) > replayEpsilon {
			return 0, fmt.Errorf("%w: event %s expected %.9f, calculated %.9f",
				ErrCostMismatch, event.EventID, event.TotalCost, calculatedCost)
		}

		totalCost += calculatedCost
	}

	return totalCost, nil
}

// ReplayResult contains the outcome of a replay comparison.
type ReplayResult struct {
	ExecutionID  string  `json:"execution_id"`
	OriginalCost float64 `json:"original_cost"`
	ReplayedCost float64 `json:"replayed_cost"`
	Delta        float64 `json:"delta"`
	Status       string  `json:"status"` // "match", "mismatch", or "error"
	Error        string  `json:"error,omitempty"`
	EventCount   int     `json:"event_count"`
}

// IsMatch returns true if the replay matched the original cost.
func (rr *ReplayResult) IsMatch() bool {
	return rr.Status == "match"
}

// CompareReplayWithOriginal compares a replay with the original ledger.
// Returns a structured result rather than a generic map.
func (r *ReplayEngine) CompareReplayWithOriginal(executionID string, originalLedger *ledger.CostLedger) ReplayResult {
	result := ReplayResult{
		ExecutionID: executionID,
	}

	if originalLedger == nil {
		result.Status = "error"
		result.Error = "ledger cannot be nil"
		return result
	}

	// Get original events and calculate original cost
	originalEvents := originalLedger.GetEventsByExecution(executionID)
	result.EventCount = len(originalEvents)

	if len(originalEvents) == 0 {
		result.Status = "error"
		result.Error = fmt.Sprintf("no events found for execution: %s", executionID)
		return result
	}

	var originalCost float64
	for _, event := range originalEvents {
		originalCost += event.TotalCost
	}
	result.OriginalCost = originalCost

	// Try to replay using current pricing
	replayedCost, err := r.replayExecution(executionID, originalLedger, false)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	result.ReplayedCost = replayedCost
	result.Delta = replayedCost - originalCost

	if math.Abs(result.Delta) < replayEpsilon {
		result.Status = "match"
	} else {
		result.Status = "mismatch"
	}

	return result
}

// ReplayAll replays all unique executions in the ledger.
// Returns a slice of results for each execution.
func (r *ReplayEngine) ReplayAll(l *ledger.CostLedger) ([]ReplayResult, error) {
	if l == nil {
		return nil, ErrNilLedger
	}

	events := l.GetEvents()
	if len(events) == 0 {
		return []ReplayResult{}, nil
	}

	// Collect unique execution IDs
	seen := make(map[string]struct{})
	var executionIDs []string
	for _, event := range events {
		if _, exists := seen[event.ExecutionID]; !exists {
			seen[event.ExecutionID] = struct{}{}
			executionIDs = append(executionIDs, event.ExecutionID)
		}
	}

	results := make([]ReplayResult, 0, len(executionIDs))
	for _, execID := range executionIDs {
		result := r.CompareReplayWithOriginal(execID, l)
		results = append(results, result)
	}

	return results, nil
}

func clonePricingModels(pricingModels map[string]models.PricingModel) map[string]models.PricingModel {
	cloned := make(map[string]models.PricingModel, len(pricingModels))
	for key, model := range pricingModels {
		cloned[key] = clonePricingModel(model)
	}
	return cloned
}

func clonePricingModel(model models.PricingModel) models.PricingModel {
	cloned := model
	if model.Tiers != nil {
		cloned.Tiers = make([]models.PricingTier, len(model.Tiers))
		copy(cloned.Tiers, model.Tiers)
	}
	if model.Metadata != nil {
		cloned.Metadata = make(map[string]string, len(model.Metadata))
		for key, value := range model.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}
