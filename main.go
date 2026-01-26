package main

import (
	"fmt"
//	"os"
	"time"

//	"icae/ledger"
//	"icae/models"
//	"icae/replay"
)

/*func createSamplePricingModels() map[string]models.PricingModel {
	// Sample token-based pricing model with tiers
	tokenPricing := models.PricingModel{
		ID:            "12345678-1234-1234-1234-123456789012",
		Version:       "v1.0.0",
		Component:     "gpt-4",
		PricingType:   "token",
		BaseUnit:      "token",
		Tiers: []models.PricingTier{
			{MinQuantity: 0, MaxQuantity: 10000, UnitCost: 0.03},
			{MinQuantity: 10000, MaxQuantity: -1, UnitCost: 0.02},
		},
		FixedFee: 0.0,
	}

	// Sample fixed fee model
	requestPricing := models.PricingModel{
		ID:            "87654321-4321-4321-4321-210987654321",
		Version:       "v1.0.0",
		Component:     "external_api",
		PricingType:   "request",
		BaseUnit:      "request",
		Tiers: []models.PricingTier{
			{MinQuantity: 0, MaxQuantity: -1, UnitCost: 0.50},
		},
		FixedFee:      0.0,
	}

	return map[string]models.PricingModel{
		"gpt-4:v1.0.0":     tokenPricing,
		"external_api:v1.0.0": requestPricing,
	}
}

func createSampleCostEvents() []models.CostEvent {
	// Create a sample execution ID
	executionID := "12345678-1234-1234-1234-123456789012"
	now := time.Now().UTC()

	// Sample model invocation event with tiered pricing
	modelEvent := models.CostEvent{
		EventID:         "11111111-1111-1111-1111-111111111111",
		Timestamp:       now,
		ExecutionID:     executionID,
		Component:       "model",
		Action:          "invoke",
		UnitCost:        0.03, // $0.03 per token
		Quantity:        1500, // 1500 tokens
		TotalCost:       45.0, // $45.00 (correct calculation)
		Currency:        "USD",
		CostSource:      "openai",
		PricingVersion:  "gpt-4:v1.0.0",
		BaseUnit:        "token",
	}

	// Sample external API call event
	apiEvent := models.CostEvent{
		EventID:         "22222222-2222-2222-2222-222222222222",
		Timestamp:       now.Add(time.Millisecond), // Ensure chronological order
		ExecutionID:     executionID,
		Component:       "external_api",
		Action:          "call",
		UnitCost:        0.50, // $0.50 per request
		Quantity:        1,    // 1 request
		TotalCost:       0.50, // $0.50 total
		Currency:        "USD",
		CostSource:      "third_party_service",
		PricingVersion:  "external_api:v1.0.0",
		BaseUnit:        "request",
	}

	return []models.CostEvent{modelEvent, apiEvent}
}
*/
func main() {
	fmt.Println("icae layer running...")
	for {
        time.Sleep(time.Hour)
    }
/*	fmt.Println("=== Inference Cost Attribution Engine (ICAE) Demo ===\n")

	// 1. Create sample pricing models
	fmt.Println("1. Creating pricing models...")
	pricingModels := createSamplePricingModels()
	for key, model := range pricingModels {
		if err := model.Validate(); err != nil {
			fmt.Printf("   ✗ Invalid pricing model %s: %v\n", key, err)
			os.Exit(1)
		}
	}
	fmt.Printf("   Created %d pricing models\n\n", len(pricingModels))

	// 2. Create cost events
	fmt.Println("2. Creating cost events...")
	events := createSampleCostEvents()
	for i, event := range events {
		if err := event.Validate(); err != nil {
			fmt.Printf("   ✗ Invalid event %d: %v\n", i, err)
			os.Exit(1)
		}
	}
	fmt.Printf("   Created %d cost events\n\n", len(events))

	// 3. Build ledger
	fmt.Println("3. Building cost ledger...")
	costLedger := ledger.NewCostLedger()
	for _, event := range events {
		if err := costLedger.AddEvent(event); err != nil {
			fmt.Printf("   ✗ Failed to add event: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("   Ledger built with hash: %s\n\n", costLedger.GetLedgerHash())

	// 4. Show total cost
	fmt.Println("4. Total cost calculation:")
	totalCost := costLedger.GetTotalCost()
	fmt.Printf("   Total cost: $%.2f\n\n", totalCost)

	// 5. Replay cost using current pricing
	fmt.Println("5. Replaying costs with current pricing...")
	replayEngine := replay.NewReplayEngine(pricingModels)
	executionID := events[0].ExecutionID
	replayedCost, err := replayEngine.ReplayExecution(executionID, costLedger)
	if err != nil {
		fmt.Printf("   ✗ Replay failed: %v\n\n", err)
	} else {
		fmt.Printf("   Reproduced cost: $%.2f\n   ✓ Replay successful - costs match\n\n", replayedCost)
	}

	// 6. Verify integrity
	fmt.Println("6. Verifying ledger integrity...")
	originalHash := costLedger.GetLedgerHash()
	if err := costLedger.VerifyIntegrity(originalHash); err != nil {
		fmt.Printf("   Ledger integrity: ✗ Invalid (%v)\n\n", err)
	} else {
		fmt.Printf("   Ledger integrity: ✓ Valid\n\n")
	}

	// 7. Show event details
	fmt.Println("7. Cost events in ledger:")
	for i, event := range costLedger.GetEvents() {
		fmt.Printf("   %d. %s (%s): $%.2f (%.0f %s)\n", i+1, event.Component, event.Action, event.TotalCost, event.Quantity, event.BaseUnit)
	}

	// 8. Compare replay with original
	fmt.Println("\n8. Replay comparison:")
	result := replayEngine.CompareReplayWithOriginal(executionID, costLedger)
	fmt.Printf("   Execution: %s\n", result.ExecutionID)
	fmt.Printf("   Original cost: $%.2f\n", result.OriginalCost)
	fmt.Printf("   Replayed cost: $%.2f\n", result.ReplayedCost)
	fmt.Printf("   Delta: $%.2f\n", result.Delta)
	fmt.Printf("   Status: %s\n", result.Status)

	fmt.Println("\n=== Demo Complete ===")
*/
	}