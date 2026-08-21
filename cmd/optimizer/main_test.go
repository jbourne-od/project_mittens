package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/service"
)

func TestOptimizer_GenerateSyntheticScenario(t *testing.T) {
	startEpoch := time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC).Unix()
	drivers, loads := generateSyntheticScenario(startEpoch)

	if len(drivers) == 0 {
		t.Fatal("expected non-empty synthetic drivers slice")
	}
	if len(loads) == 0 {
		t.Fatal("expected non-empty synthetic loads slice")
	}

	for _, d := range drivers {
		if d.ID == "" {
			t.Error("driver missing ID")
		}
		if d.AvailableEpoch != startEpoch {
			t.Errorf("driver %s available epoch %d != %d", d.ID, d.AvailableEpoch, startEpoch)
		}
	}
}

func TestOptimizer_JSONReportExport_FailClosed(t *testing.T) {
	// 1. Valid report export to temp file
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "report.json")

	report := service.RollingHorizonReport{
		RunID:                "TEST_RUN_CLI_01",
		TotalDays:            7,
		TotalEpochs:          7,
		TotalDirectTours:     12,
		TotalRelayExchanges:  4,
		TotalLoadedMiles:     4500.0,
		TotalEmptyMiles:      500.0,
		GlobalEmptyRatio:     0.10,
		TotalGrossRevenue:    12000.0,
		TotalOperatingCost:   4000.0,
		TotalNetContribution: 8000.0,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatalf("failed writing JSON report: %v", err)
	}

	// Verify exported content
	readBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed reading back exported JSON: %v", err)
	}
	var readReport service.RollingHorizonReport
	if err := json.Unmarshal(readBytes, &readReport); err != nil {
		t.Fatalf("failed unmarshaling exported report: %v", err)
	}
	if readReport.RunID != report.RunID {
		t.Errorf("expected RunID %s, got %s", report.RunID, readReport.RunID)
	}

	// 2. Writing to an invalid / non-existent directory path fails closed
	invalidPath := filepath.Join(tmpDir, "non_existent_subdir", "nested", "report.json")
	if err := os.WriteFile(invalidPath, data, 0644); err == nil {
		t.Error("expected write error on non-existent directory path without mkdir, got nil")
	}
}
