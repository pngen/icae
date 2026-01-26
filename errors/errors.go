// Package errors provides common error types and utilities for ICAE.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for ICAE operations.
var (
	// ErrValidation indicates input validation failure.
	ErrValidation = errors.New("validation error")

	// ErrChronological indicates events are not in chronological order.
	ErrChronological = errors.New("chronological order violation")

	// ErrIntegrity indicates a ledger integrity check failure.
	ErrIntegrity = errors.New("integrity check failed")

	// ErrPricing indicates a pricing calculation error.
	ErrPricing = errors.New("pricing error")

	// ErrReplay indicates a replay verification failure.
	ErrReplay = errors.New("replay error")

	// ErrAdapter indicates an adapter conversion error.
	ErrAdapter = errors.New("adapter error")

	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("not found")
)

// ICAEError wraps errors with additional context for debugging.
type ICAEError struct {
	Op      string // Operation that failed
	Kind    error  // Category of error (sentinel)
	Err     error  // Underlying error
	Context map[string]interface{}
}

func (e *ICAEError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v: %v", e.Op, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Kind)
}

func (e *ICAEError) Unwrap() error {
	return e.Err
}

func (e *ICAEError) Is(target error) bool {
	return errors.Is(e.Kind, target)
}

// Wrap creates a new ICAEError with the given context.
func Wrap(op string, kind error, err error) *ICAEError {
	return &ICAEError{Op: op, Kind: kind, Err: err}
}