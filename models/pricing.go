package models

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// PricingTier represents a pricing tier for a model or service.
type PricingTier struct {
	MinQuantity float64
	MaxQuantity float64 // -1 means no upper bound
	UnitCost    float64
}

// Epsilon for floating-point comparisons throughout pricing calculations.
const pricingEpsilon = 1e-9

// PricingModel defines how costs are calculated.
type PricingModel struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Component   string            `json:"component"`
	PricingType string            `json:"pricing_type"`
	BaseUnit    string            `json:"base_unit"`
	Tiers       []PricingTier     `json:"tiers"`
	FixedFee    float64           `json:"fixed_fee,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Validate checks that the PricingModel is correctly configured.
func (p *PricingModel) Validate() error {
	if p.ID == "" {
		return errors.New("pricing model ID is required")
	}
	if p.Version == "" {
		return errors.New("pricing model version is required")
	}
	if p.Component == "" {
		return errors.New("pricing model component is required")
	}
	if p.PricingType == "" {
		return errors.New("pricing model pricing_type is required")
	}
	if p.BaseUnit == "" {
		return errors.New("pricing model base_unit is required")
	}
	if !isFinite(p.FixedFee) {
		return errors.New("fixed_fee must be finite")
	}
	if p.FixedFee < 0 {
		return errors.New("fixed_fee cannot be negative")
	}

	// Validate tiers if present
	if len(p.Tiers) > 0 {
		if err := p.validateTiers(); err != nil {
			return err
		}
	}

	return nil
}

// validateTiers ensures tier configuration is valid and contiguous.
func (p *PricingModel) validateTiers() error {
	sortedTiers := make([]PricingTier, len(p.Tiers))
	copy(sortedTiers, p.Tiers)
	sort.Slice(sortedTiers, func(i, j int) bool {
		return sortedTiers[i].MinQuantity < sortedTiers[j].MinQuantity
	})

	expectedMin := 0.0
	for i, tier := range sortedTiers {
		if !isFinite(tier.MinQuantity) {
			return fmt.Errorf("tier %d: min_quantity must be finite", i)
		}
		if tier.MaxQuantity != -1 && !isFinite(tier.MaxQuantity) {
			return fmt.Errorf("tier %d: max_quantity must be finite or -1 for unbounded", i)
		}
		if !isFinite(tier.UnitCost) {
			return fmt.Errorf("tier %d: unit_cost must be finite", i)
		}
		if tier.MinQuantity < 0 {
			return fmt.Errorf("tier %d: min_quantity cannot be negative", i)
		}
		if tier.MaxQuantity != -1 && tier.MaxQuantity < tier.MinQuantity {
			return fmt.Errorf("tier %d: max_quantity must be >= min_quantity or -1 for unbounded", i)
		}
		if tier.UnitCost < 0 {
			return fmt.Errorf("tier %d: unit_cost cannot be negative", i)
		}
		if math.Abs(tier.MinQuantity-expectedMin) > pricingEpsilon {
			return fmt.Errorf("tier %d: min_quantity %.9f must equal expected contiguous start %.9f", i, tier.MinQuantity, expectedMin)
		}
		if tier.MaxQuantity == -1 {
			if i != len(sortedTiers)-1 {
				return fmt.Errorf("tier %d: unbounded tier must be the final tier", i)
			}
			continue
		}
		expectedMin = tier.MaxQuantity
	}
	return nil
}

// CalculateCost calculates cost for a given quantity based on the pricing tiers.
// For fixed-fee only models (no tiers), returns just the fixed fee.
// Returns an error if quantity is negative or no tier covers the quantity.
func (p *PricingModel) CalculateCost(quantity float64) (float64, error) {
	if err := p.Validate(); err != nil {
		return 0, fmt.Errorf("invalid pricing model: %w", err)
	}
	if quantity < 0 {
		return 0, fmt.Errorf("quantity cannot be negative: got %.9f", quantity)
	}
	if !isFinite(quantity) {
		return 0, fmt.Errorf("quantity must be finite: got %.9f", quantity)
	}

	// Handle zero quantity - return only fixed fee
	if quantity < pricingEpsilon {
		return p.FixedFee, nil
	}

	totalCost := p.FixedFee

	// Fixed-fee only model (no tiers) - valid for request-based pricing
	if len(p.Tiers) == 0 {
		return totalCost, nil
	}

	// Sort tiers by min_quantity to ensure correct processing
	sortedTiers := make([]PricingTier, len(p.Tiers))
	copy(sortedTiers, p.Tiers)
	for i := range sortedTiers {
		for j := i + 1; j < len(sortedTiers); j++ {
			if sortedTiers[i].MinQuantity > sortedTiers[j].MinQuantity {
				sortedTiers[i], sortedTiers[j] = sortedTiers[j], sortedTiers[i]
			}
		}
	}

	// Validate that tiers cover the quantity
	if quantity < sortedTiers[0].MinQuantity {
		return 0, fmt.Errorf("no applicable pricing tier for quantity %.9f (first tier starts at %.9f)", quantity, sortedTiers[0].MinQuantity)
	}

	remainingQuantity := quantity
	processedUpTo := 0.0

	for _, tier := range sortedTiers {
		// If we've processed all quantity, we're done
		if remainingQuantity <= 0 {
			break
		}

		// Skip tiers that start after our quantity
		if tier.MinQuantity > quantity {
			break
		}

		// Calculate how much quantity falls into this tier
		tierStart := math.Max(tier.MinQuantity, processedUpTo)
		var tierEnd float64
		if tier.MaxQuantity == -1 {
			tierEnd = math.Inf(1)
		} else {
			tierEnd = tier.MaxQuantity
		}

		// How much of our quantity falls into this tier?
		quantityInTier := math.Min(quantity, tierEnd) - tierStart

		if quantityInTier > 0 {
			totalCost += quantityInTier * tier.UnitCost
			processedUpTo = math.Min(quantity, tierEnd)
			remainingQuantity -= quantityInTier
		}
	}

	// If we still have remaining quantity, no tier covered it
	if remainingQuantity > pricingEpsilon {
		return 0, fmt.Errorf("no applicable pricing tier for remaining quantity %.9f", remainingQuantity)
	}

	return totalCost, nil
}

// Key returns the canonical key for this pricing model (component:version).
func (p *PricingModel) Key() string {
	return fmt.Sprintf("%s:%s", p.Component, p.Version)
}
