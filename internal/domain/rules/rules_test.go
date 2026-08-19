package rules_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/rules"
)

func TestRules_ConditionAndBonusEvaluation(t *testing.T) {
	ruleDefs := []rules.Rule{
		{
			ID:           "RULE-FLATBED-LONGHAUL-BONUS",
			Name:         "Flatbed Longhaul Bonus",
			Target:       rules.TargetBonus,
			Operation:    rules.OpAdd,
			ConditionCEL: "driver.equipment_type == 'FLATBED_53' && arc.loaded_miles >= 500.0",
			ValueCEL:     "arc.loaded_miles * 0.25", // $0.25/mile bonus for longhaul flatbed
		},
		{
			ID:           "RULE-REEFER-FIXED-BONUS",
			Name:         "Reefer Priority Bonus",
			Target:       rules.TargetBonus,
			Operation:    rules.OpAdd,
			ConditionCEL: "load.required_equipment == 'REEFER_53'",
			StaticValue:  150.0,
		},
	}

	reg, err := rules.NewRuleRegistry(ruleDefs, nil)
	if err != nil {
		t.Fatalf("NewRuleRegistry failed: %v", err)
	}

	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	// Scenario 1: Flatbed driver with 600-mile dry van load (matches flatbed rule)
	driverFlatbed := model.Driver{
		ID:              "D-FB-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipFlatbed},
	}
	loadLonghaul := model.Load{
		ID:                  "L-01",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipFlatbed,
		Revenue:             3000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	ctxMap1 := rules.BuildEvaluationContext(driverFlatbed, loadLonghaul, 50.0, 600.0)
	res1, err := reg.Evaluate(context.Background(), ctxMap1)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(res1.MatchedRuleIDs) != 1 || res1.MatchedRuleIDs[0] != "RULE-FLATBED-LONGHAUL-BONUS" {
		t.Errorf("expected RULE-FLATBED-LONGHAUL-BONUS to match, got %v", res1.MatchedRuleIDs)
	}
	expectedBonus := 600.0 * 0.25 // $150.00
	if res1.Bonus != expectedBonus {
		t.Errorf("expected bonus %f, got %f", expectedBonus, res1.Bonus)
	}

	// Scenario 2: Dry Van driver with Reefer load
	driverVan := model.Driver{
		ID:              "D-VAN-01",
		CurrentLocation: locChi,
		AvailableEpoch:  startEpoch,
		Equipment:       model.Equipment{Type: model.EquipDryVan},
	}
	loadReefer := model.Load{
		ID:                  "L-02",
		Origin:              locChi,
		Destination:         locAtl,
		RequiredEquipment:   model.EquipReefer,
		Revenue:             3000.0,
		PickupEarliestEpoch: startEpoch,
		PickupLatestEpoch:   startEpoch + 36000,
		DeliveryLatestEpoch: startEpoch + 120000,
	}

	ctxMap2 := rules.BuildEvaluationContext(driverVan, loadReefer, 20.0, 300.0)
	res2, err := reg.Evaluate(context.Background(), ctxMap2)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(res2.MatchedRuleIDs) != 1 || res2.MatchedRuleIDs[0] != "RULE-REEFER-FIXED-BONUS" {
		t.Errorf("expected RULE-REEFER-FIXED-BONUS to match, got %v", res2.MatchedRuleIDs)
	}
	if res2.Bonus != 150.0 {
		t.Errorf("expected bonus 150.0, got %f", res2.Bonus)
	}
}

func TestRules_InfeasibilityBanning(t *testing.T) {
	ruleDefs := []rules.Rule{
		{
			ID:           "RULE-BAN-CHICAGO-NEWYORK",
			Name:         "Ban Chicago to New York Lane",
			Target:       rules.TargetInfeasible,
			Operation:    rules.OpBan,
			ConditionCEL: "driver.current_location_id == 'CHI' && load.destination_id == 'NYC'",
			Message:      "Chicago to New York dispatches are prohibited by operational policy",
		},
	}

	reg, err := rules.NewRuleRegistry(ruleDefs, nil)
	if err != nil {
		t.Fatalf("NewRuleRegistry failed: %v", err)
	}

	driver := model.Driver{
		ID:              "D-01",
		CurrentLocation: model.Location{NodeID: "CHI"},
	}
	loadNYC := model.Load{
		ID:          "L-NYC",
		Origin:      model.Location{NodeID: "CHI"},
		Destination: model.Location{NodeID: "NYC"},
	}
	loadATL := model.Load{
		ID:          "L-ATL",
		Origin:      model.Location{NodeID: "CHI"},
		Destination: model.Location{NodeID: "ATL"},
	}

	// NYC load should be banned
	ctxNYC := rules.BuildEvaluationContext(driver, loadNYC, 0.0, 800.0)
	resNYC, err := reg.Evaluate(context.Background(), ctxNYC)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !resNYC.IsInfeasible {
		t.Errorf("expected NYC load to be infeasible")
	}
	if resNYC.InfeasibilityReason != "Chicago to New York dispatches are prohibited by operational policy" {
		t.Errorf("unexpected reason: %s", resNYC.InfeasibilityReason)
	}

	// ATL load should NOT be banned
	ctxATL := rules.BuildEvaluationContext(driver, loadATL, 0.0, 600.0)
	resATL, err := reg.Evaluate(context.Background(), ctxATL)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if resATL.IsInfeasible {
		t.Errorf("expected ATL load to be feasible")
	}
}

func TestRules_CostMultipliers(t *testing.T) {
	ruleDefs := []rules.Rule{
		{
			ID:           "RULE-WEEKEND-SURCHARGE",
			Name:         "Weekend Loaded Rate Surcharge",
			Target:       rules.TargetLoadedRate,
			Operation:    rules.OpMultiply,
			ConditionCEL: "arc.loaded_miles > 1000.0",
			StaticValue:  1.20, // 20% surcharge on loaded rate
		},
	}

	reg, err := rules.NewRuleRegistry(ruleDefs, nil)
	if err != nil {
		t.Fatalf("NewRuleRegistry failed: %v", err)
	}

	driver := model.Driver{ID: "D-01"}
	load := model.Load{ID: "L-01"}

	// Over 1000 miles
	ctxLong := rules.BuildEvaluationContext(driver, load, 50.0, 1200.0)
	resLong, _ := reg.Evaluate(context.Background(), ctxLong)
	if resLong.LoadedRateMultiplier != 1.20 {
		t.Errorf("expected 1.20 multiplier, got %f", resLong.LoadedRateMultiplier)
	}

	// Under 1000 miles
	ctxShort := rules.BuildEvaluationContext(driver, load, 50.0, 800.0)
	resShort, _ := reg.Evaluate(context.Background(), ctxShort)
	if resShort.LoadedRateMultiplier != 1.0 {
		t.Errorf("expected 1.0 multiplier, got %f", resShort.LoadedRateMultiplier)
	}
}

func TestRules_CompileValidationErrors(t *testing.T) {
	// 1. Invalid CEL syntax
	badSyntax := []rules.Rule{
		{
			ID:           "RULE-BAD-SYNTAX",
			ConditionCEL: "driver.id === 'invalid_syntax'",
		},
	}
	if _, err := rules.NewRuleRegistry(badSyntax, nil); err == nil {
		t.Errorf("expected compile error on invalid CEL syntax, got nil")
	}

	// 2. Non-boolean condition return type
	nonBool := []rules.Rule{
		{
			ID:           "RULE-NON-BOOL",
			ConditionCEL: "driver.drive_hours_remaining + 5.0",
		},
	}
	if _, err := rules.NewRuleRegistry(nonBool, nil); err == nil {
		t.Errorf("expected compile error on non-bool condition, got nil")
	}

	// 3. Non-numeric / non-string value return type (e.g. list literal)
	badValue := []rules.Rule{
		{
			ID:           "RULE-BAD-VAL",
			ConditionCEL: "driver.id == 'D-01'",
			ValueCEL:     "[1, 2, 3]", // list type instead of numeric/string
		},
	}
	if _, err := rules.NewRuleRegistry(badValue, nil); err == nil {
		t.Errorf("expected compile error on invalid value type, got nil")
	}
}

func TestRules_ConcurrentEvaluationRaceDetector(t *testing.T) {
	ruleDefs := []rules.Rule{
		{
			ID:           "RULE-CONCURRENT-1",
			Target:       rules.TargetBonus,
			Operation:    rules.OpAdd,
			ConditionCEL: "arc.loaded_miles > 500.0",
			ValueCEL:     "arc.loaded_miles * 0.10",
		},
		{
			ID:           "RULE-CONCURRENT-2",
			Target:       rules.TargetLoadedRate,
			Operation:    rules.OpMultiply,
			ConditionCEL: "driver.equipment_type == 'FLATBED'",
			StaticValue:  1.15,
		},
	}

	reg, err := rules.NewRuleRegistry(ruleDefs, nil)
	if err != nil {
		t.Fatalf("NewRuleRegistry failed: %v", err)
	}

	const goroutines = 32
	const iterations = 100

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			eqType := model.EquipDryVan
			if gID%2 == 0 {
				eqType = model.EquipFlatbed
			}
			driver := model.Driver{
				ID:        "D-CONC",
				Equipment: model.Equipment{Type: eqType},
			}
			load := model.Load{
				ID: "L-CONC",
			}
			ctxMap := rules.BuildEvaluationContext(driver, load, 50.0, float64(400+gID*10))

			for i := 0; i < iterations; i++ {
				res, err := reg.Evaluate(context.Background(), ctxMap)
				if err != nil {
					t.Errorf("goroutine %d iteration %d failed: %v", gID, i, err)
					return
				}
				_ = res
			}
		}(g)
	}

	wg.Wait()
}
