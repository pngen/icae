package ledger

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
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
	ErrDuplicateEventID   = errors.New("duplicate event_id")
	ErrCurrencyMismatch   = errors.New("event currency does not match ledger currency")
)

// CostLedger represents a tamper-evident, append-only ledger for cost events.
type CostLedger struct {
	mu      sync.RWMutex
	events  []models.CostEvent
	hashes  []string
	hashErr error // Tracks any error during hash computation
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
	// Strip location and monotonic-clock state so timestamps have one canonical
	// representation in both the observable ledger and its hash.
	event.Timestamp = event.Timestamp.UTC()

	// Validate the event before acquiring lock
	if err := event.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrEventValidation, err)
	}
	storedEvent := cloneEvent(event)

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, existing := range l.events {
		if existing.EventID == event.EventID {
			return fmt.Errorf("%w: %s", ErrDuplicateEventID, event.EventID)
		}
	}

	if len(l.events) > 0 {
		lastEvent := l.events[len(l.events)-1]
		if event.Timestamp.Before(lastEvent.Timestamp) {
			return fmt.Errorf("%w: event timestamp %v is before last event %v",
				ErrChronologicalOrder, event.Timestamp, lastEvent.Timestamp)
		}
		if event.Timestamp.Equal(lastEvent.Timestamp) && event.EventID < lastEvent.EventID {
			return fmt.Errorf("%w: event_id %q must sort after %q when timestamps are equal",
				ErrChronologicalOrder, event.EventID, lastEvent.EventID)
		}
		if event.Currency != l.events[0].Currency {
			return fmt.Errorf("%w: got %q, ledger uses %q",
				ErrCurrencyMismatch, event.Currency, l.events[0].Currency)
		}
	}

	l.events = append(l.events, storedEvent)
	if err := l.updateHashLocked(); err != nil {
		// Rollback the event addition on hash failure
		l.events = l.events[:len(l.events)-1]
		return fmt.Errorf("failed to update ledger hash: %w", err)
	}
	return nil
}

// updateHashLocked updates the ledger's hash. Caller must hold l.mu.
func (l *CostLedger) updateHashLocked() error {
	// Events are admitted in canonical (timestamp, event_id) order and stored
	// with UTC timestamps. Encoding the event structs directly preserves exact
	// accepted float64 values, including fixed fees, without decimal rounding.
	dataBytes, err := json.Marshal(l.events)
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
	for i, event := range l.events {
		result[i] = cloneEvent(event)
	}
	return result
}

// GetEventsByExecution filters events by execution ID.
func (l *CostLedger) GetEventsByExecution(executionID string) []models.CostEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []models.CostEvent
	for _, event := range l.events {
		if event.ExecutionID == executionID {
			filtered = append(filtered, cloneEvent(event))
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
			filtered = append(filtered, cloneEvent(event))
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
	for i, event := range l.events {
		events[i] = cloneEvent(event)
	}

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

func cloneEvent(event models.CostEvent) models.CostEvent {
	event.Metadata = cloneMetadata(event.Metadata)
	return event
}

func cloneMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}
	cloned := cloneMetadataValue(reflect.ValueOf(metadata), make(map[cloneVisit]reflect.Value))
	return cloned.Interface().(map[string]interface{})
}

type cloneVisit struct {
	typeOf   reflect.Type
	pointer  uintptr
	length   int
	capacity int
}

// cloneMetadataValue recursively clones reference-bearing JSON-compatible Go
// values while preserving their concrete types. The visited table also keeps
// shared references shared and prevents cycles from recursing indefinitely;
// encoding/json will reject unsupported cyclic metadata when the hash is made.
func cloneMetadataValue(value reflect.Value, visited map[cloneVisit]reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedValue := cloneMetadataValue(value.Elem(), visited)
		clonedInterface := reflect.New(value.Type()).Elem()
		clonedInterface.Set(clonedValue)
		return clonedInterface

	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := cloneVisit{typeOf: value.Type(), pointer: uintptr(value.UnsafePointer())}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		clonedPointer := reflect.New(value.Type().Elem())
		visited[visit] = clonedPointer
		clonedPointer.Elem().Set(cloneMetadataValue(value.Elem(), visited))
		return clonedPointer

	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := cloneVisit{typeOf: value.Type(), pointer: uintptr(value.UnsafePointer())}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		clonedMap := reflect.MakeMapWithSize(value.Type(), value.Len())
		visited[visit] = clonedMap
		iterator := value.MapRange()
		for iterator.Next() {
			clonedKey := cloneMetadataValue(iterator.Key(), visited)
			clonedValue := cloneMetadataValue(iterator.Value(), visited)
			clonedMap.SetMapIndex(clonedKey, clonedValue)
		}
		return clonedMap

	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := cloneVisit{
			typeOf:   value.Type(),
			pointer:  uintptr(value.UnsafePointer()),
			length:   value.Len(),
			capacity: value.Cap(),
		}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		clonedSlice := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		visited[visit] = clonedSlice
		for i := 0; i < value.Len(); i++ {
			clonedSlice.Index(i).Set(cloneMetadataValue(value.Index(i), visited))
		}
		return clonedSlice

	case reflect.Array:
		clonedArray := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			clonedArray.Index(i).Set(cloneMetadataValue(value.Index(i), visited))
		}
		return clonedArray

	case reflect.Struct:
		// Start with a value copy so unexported scalar state is preserved, then
		// replace exported reference-bearing fields with defensive clones.
		clonedStruct := reflect.New(value.Type()).Elem()
		clonedStruct.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath != "" {
				continue
			}
			clonedStruct.Field(i).Set(cloneMetadataValue(value.Field(i), visited))
		}
		return clonedStruct

	default:
		return value
	}
}
