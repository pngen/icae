package ledger

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"errors"
	"sync"
	"sort"
	"time"

	"icae/models"
)

// ledgerEpsilon is used for floating-point comparisons in cost calculations.
const ledgerEpsilon = 1e-9

// Sentinel errors for ledger operations.
var (
	ErrChronologicalOrder = errors.New("events must be added in chronological order")
	ErrEmptyLedger        = errors.New("ledger contains no events")
	ErrHashMismatch       = errors.New("ledger hash does not match expected value")
	ErrEventValidation    = errors.New("event validation failed")
)

// CostLedger represents a tamper-evident, append-only ledger for cost events.
type CostLedger struct {
	mu       sync.RWMutex
	events   []models.CostEvent
	hashes   []string
	hashErr  error // Tracks any error during hash computation
}

// NewCostLedger creates and returns a new CostLedger instance.
func NewCostLedger() *CostLedger {
	return &CostLedger{
		events: []models.CostEvent{},
		hashes: []string{},
	}
}

// Returns an error if the event fails validation or violates chronological ordering.
func (l *CostLedger) AddEvent(event models.CostEvent) error {
	// Validate the event before acquiring lock
	if err := event.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrEventValidation, err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure events are added in chronological order
	if len(l.events) > 0 && event.Timestamp.Before(l.events[len(l.events)-1].Timestamp) {
		return fmt.Errorf("%w: event timestamp %v is before last event %v",
			ErrChronologicalOrder, event.Timestamp, l.events[len(l.events)-1].Timestamp)
	}

	l.events = append(l.events, event)
	if err := l.updateHashLocked(); err != nil {
		// Rollback the event addition on hash failure
		l.events = l.events[:len(l.events)-1]
		return fmt.Errorf("failed to update ledger hash: %w", err)
	}
	return nil
}

// updateHashLocked updates the ledger's hash. Caller must hold l.mu.
func (l *CostLedger) updateHashLocked() error {
	// Create a deterministic representation of all events
	eventData := make([]map[string]interface{}, len(l.events))
	for i, e := range l.events {
		eventData[i] = map[string]interface{}{
			"event_id":         e.EventID,
			"timestamp":        e.Timestamp.Format(time.RFC3339Nano),
			"execution_id":     e.ExecutionID,
			"component":        e.Component,
			"action":           e.Action,
			"unit_cost":        fmt.Sprintf("%.9f", e.UnitCost),
			"quantity":         fmt.Sprintf("%.9f", e.Quantity),
			"total_cost":       fmt.Sprintf("%.9f", e.TotalCost),
			"currency":         e.Currency,
			"cost_source":      e.CostSource,
			"pricing_version":  e.PricingVersion,
			"base_unit":        e.BaseUnit,
		}
		// Only include metadata if non-nil to ensure determinism
		if e.Metadata != nil {
			eventData[i]["metadata"] = e.Metadata
		}
	}

	// Sort by event_id for deterministic ordering (timestamp may have duplicates)
	sort.Slice(eventData, func(i, j int) bool {
		ti := eventData[i]["timestamp"].(string)
		tj := eventData[j]["timestamp"].(string)
		if ti != tj {
			return ti < tj
		}
		return eventData[i]["event_id"].(string) < eventData[j]["event_id"].(string)
	})

	dataBytes, err := json.Marshal(eventData)
	if err != nil {
		l.hashErr = fmt.Errorf("failed to marshal event data: %w", err)
		return l.hashErr
	}

	hash := sha256.Sum256(dataBytes)
	l.hashes = append(l.hashes, fmt.Sprintf("%x", hash))
	l.hashErr = nil
	return nil
}

// Returns a copy to prevent external modification.
func (l *CostLedger) GetEvents() []models.CostEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]models.CostEvent, len(l.events))
	copy(result, l.events)
	return result
}

// GetEventsByExecution filters events by execution ID.
func (l *CostLedger) GetEventsByExecution(executionID string) []models.CostEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []models.CostEvent
	for _, event := range l.events {
		if event.ExecutionID == executionID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// GetEventsByComponent filters events by component.
func (l *CostLedger) GetEventsByComponent(component string) []models.CostEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []models.CostEvent
	for _, event := range l.events {
		if event.Component == component {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// GetTotalCost calculates the total cost across all events.
func (l *CostLedger) GetTotalCost() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var total float64
	for _, event := range l.events {
		total += event.TotalCost
	}
	return total
}

// GetEventCount returns the number of events in the ledger.
func (l *CostLedger) GetEventCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// GetLedgerHash returns the current hash of the ledger.
func (l *CostLedger) GetLedgerHash() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.hashes) == 0 {
		// Return hash of empty ledger for consistency
		emptyBytes := []byte("[]")
		emptyHash := sha256.Sum256(emptyBytes)
		return fmt.Sprintf("%x", emptyHash)
	}
	return l.hashes[len(l.hashes)-1]
}

// GetHashHistory returns all historical hashes for audit purposes.
func (l *CostLedger) GetHashHistory() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]string, len(l.hashes))
	copy(result, l.hashes)
	return result
}

// VerifyIntegrity checks if the ledger matches an expected hash and returns an error if not.
func (l *CostLedger) VerifyIntegrity(expectedHash string) error {
	currentHash := l.GetLedgerHash()
	if currentHash != expectedHash {
		return fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, expectedHash, currentHash)
	}
	return nil
}

// VerifyIntegrityBool is a convenience method that returns a boolean.
// Deprecated: Use VerifyIntegrity for better error handling.
func (l *CostLedger) VerifyIntegrityBool(expectedHash string) bool {
	return l.VerifyIntegrity(expectedHash) == nil
}

// ReplayCost recomputes total cost for a specific execution.
func (l *CostLedger) ReplayCost(executionID string) float64 {
	events := l.GetEventsByExecution(executionID)
	var total float64
	for _, event := range events {
		total += event.TotalCost
	}
	return total
}

// Snapshot returns a serializable representation of the ledger state.
func (l *CostLedger) Snapshot() LedgerSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()

	events := make([]models.CostEvent, len(l.events))
	copy(events, l.events)

	hashes := make([]string, len(l.hashes))
	copy(hashes, l.hashes)

	// Compute current hash inline to avoid deadlock (GetLedgerHash also acquires RLock)
	var currentHash string
	if len(l.hashes) == 0 {
		emptyBytes := []byte("[]")
		emptyHash := sha256.Sum256(emptyBytes)
		currentHash = fmt.Sprintf("%x", emptyHash)
	} else {
		currentHash = l.hashes[len(l.hashes)-1]
	}

	return LedgerSnapshot{
		Events:      events,
		Hashes:      hashes,
		SnapshotAt:  time.Now().UTC(),
		EventCount:  len(events),
		CurrentHash: currentHash,
	}
}

// LedgerSnapshot represents a point-in-time snapshot of the ledger.
type LedgerSnapshot struct {
	Events      []models.CostEvent `json:"events"`
	Hashes      []string           `json:"hashes"`
	SnapshotAt  time.Time          `json:"snapshot_at"`
	EventCount  int                `json:"event_count"`
	CurrentHash string             `json:"current_hash"`
}