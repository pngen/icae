package models

import (
	"math"
	"testing"
)

func TestPricingModel_Validate(t *testing.T) {
	tests := []struct {
		name    string
		model   PricingModel
		wantErr bool
	}{
		{
			name: "valid model with tiers",
			model: PricingModel{
				ID:          "pm-001",
				Version:     "v1.0.0",
				Component:   "gpt-4",
				PricingType: "token",
				BaseUnit:    "token",
				Tiers: []PricingTier{
					{MinQuantity: 0, MaxQuantity: 1000, UnitCost: 0.03},
				},
			},
			wantErr: false,
		},
		{
			name: "valid fixed-fee only",
			model: PricingModel{
				ID:          "pm-002",
				Version:     "v1.0.0",
				Component:   "api",
				PricingType: "request",
				BaseUnit:    "request",
				FixedFee:    0.50,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			model: PricingModel{
				Version:     "v1.0.0",
				Component:   "gpt-4",
				PricingType: "token",
				BaseUnit:    "token",
			},
			wantErr: true,
		},
		{
			name: "negative fixed fee",
			model: PricingModel{
				ID:          "pm-003",
				Version:     "v1.0.0",
				Component:   "api",
				PricingType: "request",
				BaseUnit:    "request",
				FixedFee:    -0.50,
			},
			wantErr: true,
		},
		{
			name: "invalid tier - negative min",
			model: PricingModel{
				ID:          "pm-004",
				Version:     "v1.0.0",
				Component:   "gpt-4",
				PricingType: "token",
				BaseUnit:    "token",
				Tiers: []PricingTier{
					{MinQuantity: -1, MaxQuantity: 1000, UnitCost: 0.03},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.model.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPricingModel_CalculateCost(t *testing.T) {
	tieredModel := PricingModel{
		ID:          "pm-001",
		Version:     "v1.0.0",
		Component:   "gpt-4",
		PricingType: "token",
		BaseUnit:    "token",
		Tiers: []PricingTier{
			{MinQuantity: 0, MaxQuantity: 1000, UnitCost: 0.03},
			{MinQuantity: 1000, MaxQuantity: -1, UnitCost: 0.02},
		},
		FixedFee: 1.00,
	}

	fixedFeeModel := PricingModel{
		ID:          "pm-002",
		Version:     "v1.0.0",
		Component:   "api",
		PricingType: "request",
		BaseUnit:    "request",
		FixedFee:    0.50,
	}

	tests := []struct {
		name     string
		model    PricingModel
		quantity float64
		want     float64
		wantErr  bool
	}{
		{
			name:     "tiered - first tier only",
			model:    tieredModel,
			quantity: 500,
			want:     1.00 + (500 * 0.03), // fixed + tier1
			wantErr:  false,
		},
		{
			name:     "tiered - spans two tiers",
			model:    tieredModel,
			quantity: 1500,
			want:     1.00 + (1000 * 0.03) + (500 * 0.02), // fixed + tier1 + tier2
			wantErr:  false,
		},
		{
			name:     "fixed fee only - zero quantity",
			model:    fixedFeeModel,
			quantity: 0,
			want:     0.50,
			wantErr:  false,
		},
		{
			name:     "negative quantity",
			model:    tieredModel,
			quantity: -100,
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.model.CalculateCost(tt.quantity)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateCost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && math.Abs(got-tt.want) > pricingEpsilon {
				t.Errorf("CalculateCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPricingModel_Key(t *testing.T) {
	model := PricingModel{
		Component: "gpt-4",
		Version:   "v1.0.0",
	}
	want := "gpt-4:v1.0.0"
	if got := model.Key(); got != want {
		t.Errorf("Key() = %v, want %v", got, want)
	}
}