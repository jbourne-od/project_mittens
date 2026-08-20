package journal_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
	"github.com/optimaldynamics/project-mittens/pkg/journal"
)

func createTestState() *model.State[model.AggregatedMarket] {
	locChi := model.Location{NodeID: "CHI", Lat: 41.8781, Lon: -87.6298}
	locAtl := model.Location{NodeID: "ATL", Lat: 33.7490, Lon: -84.3880}
	startEpoch := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC).Unix()

	drivers := []model.Driver{
		{
			ID:                  "DRV-02",
			CurrentLocation:     locChi,
			HomeLocation:        locChi,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 11.0,
			DutyHoursRemaining:  14.0,
			Equipment:           model.Equipment{Type: model.EquipDryVan},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
		{
			ID:                  "DRV-01",
			CurrentLocation:     locAtl,
			HomeLocation:        locAtl,
			AvailableEpoch:      startEpoch,
			DriveHoursRemaining: 10.0,
			DutyHoursRemaining:  12.0,
			Equipment:           model.Equipment{Type: model.EquipReefer},
			Clocks:              hos.NewDriverClocks(hos.USPolicySpecs(), time.Unix(startEpoch, 0)),
		},
	}

	loads := []model.Load{
		{
			ID:                    "LOAD-02",
			Origin:                locChi,
			Destination:           locAtl,
			RequiredEquipment:     model.EquipDryVan,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 18000,
			DeliveryLatestEpoch:   startEpoch + 120000,
			Revenue:               2500.0,
		},
		{
			ID:                    "LOAD-01",
			Origin:                locAtl,
			Destination:           locChi,
			RequiredEquipment:     model.EquipReefer,
			PickupEarliestEpoch:   startEpoch,
			PickupLatestEpoch:     startEpoch + 36000,
			DeliveryEarliestEpoch: startEpoch + 18000,
			DeliveryLatestEpoch:   startEpoch + 120000,
			Revenue:               2800.0,
		},
	}

	res := model.NewResourceState(drivers, loads)
	info, _ := model.NewInformationState(startEpoch, 2.50, 3.50, 2)
	scale := model.AggregatedMarket{}
	states := []string{"AGGRESSIVE", "MODERATE", "PASSIVE"}
	belief, _ := model.NewBelief(scale, states, []float64{0.20, 0.50, 0.30})

	state, _ := model.NewState(res, info, belief)
	return state
}

func TestCanonicalHashing_Determinism(t *testing.T) {
	state1 := createTestState()
	state2 := createTestState()

	hash1, err1 := journal.HashState(state1)
	if err1 != nil {
		t.Fatalf("HashState 1 failed: %v", err1)
	}

	hash2, err2 := journal.HashState(state2)
	if err2 != nil {
		t.Fatalf("HashState 2 failed: %v", err2)
	}

	if hash1 != hash2 {
		t.Fatalf("hashes should be bit-exact identical: got %s vs %s", hash1, hash2)
	}

	// Verify action hashing
	matches := []model.DriverLoadMatch{
		{DriverID: "DRV-02", LoadID: "LOAD-02"},
		{DriverID: "DRV-01", LoadID: "LOAD-01"},
	}
	bids := []model.SpotPriceBid{
		{LoadID: "LOAD-02", BidPrice: 2400.0},
		{LoadID: "LOAD-01", BidPrice: 2700.0},
	}

	action1 := model.NewAction(matches, bids)
	action2 := model.NewAction(matches, bids)

	_, aHash1, _ := journal.EncodeCanonicalAction(action1)
	_, aHash2, _ := journal.EncodeCanonicalAction(action2)

	if aHash1 != aHash2 {
		t.Fatalf("action hashes must match: %s vs %s", aHash1, aHash2)
	}
}

func TestMemoryStore_MerkleChainIntegrityAndTamperDetection(t *testing.T) {
	store := journal.NewMemoryStore()
	runID := "RUN-TEST-001"

	// Record 1 (Genesis)
	rec1 := journal.JournalRecord{
		RunID:                runID,
		Epoch:                1000,
		BatchSeq:             1,
		DecisionID:           "DEC-001",
		RuntimeVersion:       journal.CurrentRuntimeVersion,
		PolicyName:           "CFA",
		PolicyParamHash:      "hash-p1",
		InitialStateHash:     "hash-s1",
		ActionHash:           "hash-a1",
		NextStateHash:        "hash-s2",
		TotalNetContribution: 1500.0,
	}
	rec1.Seal()

	if err := store.Append(rec1); err != nil {
		t.Fatalf("failed appending rec1: %v", err)
	}

	// Record 2 (Linked to rec1)
	rec2 := journal.JournalRecord{
		RunID:                runID,
		Epoch:                2000,
		BatchSeq:             2,
		DecisionID:           "DEC-002",
		PrevRecordHash:       rec1.RecordHash,
		RuntimeVersion:       journal.CurrentRuntimeVersion,
		PolicyName:           "CFA",
		PolicyParamHash:      "hash-p1",
		InitialStateHash:     "hash-s2",
		ActionHash:           "hash-a2",
		NextStateHash:        "hash-s3",
		TotalNetContribution: 1800.0,
	}
	rec2.Seal()

	if err := store.Append(rec2); err != nil {
		t.Fatalf("failed appending rec2: %v", err)
	}

	// Verify valid chain
	valid, lastHash, err := store.VerifyRunChain(runID)
	if err != nil || !valid {
		t.Fatalf("chain should be valid: %v", err)
	}
	if lastHash != rec2.RecordHash {
		t.Errorf("expected last hash %s, got %s", rec2.RecordHash, lastHash)
	}

	// Test Tamper Detection: Append record with corrupted prev hash
	tamperedRec := journal.JournalRecord{
		RunID:            runID,
		Epoch:            3000,
		BatchSeq:         3,
		DecisionID:       "DEC-003",
		PrevRecordHash:   "corrupted-prev-hash",
		RuntimeVersion:   journal.CurrentRuntimeVersion,
		PolicyName:       "CFA",
		InitialStateHash: "hash-s3",
		ActionHash:       "hash-a3",
		NextStateHash:    "hash-s4",
	}
	tamperedRec.Seal()

	if err := store.Append(tamperedRec); err == nil {
		t.Errorf("expected error appending tampered record with mismatched prev hash")
	}
}

func TestFileStore_PersistenceAndReload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "journal_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "semantic_journal.jsonl")

	// Create and write to FileStore
	fs1, err := journal.NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	rec := journal.JournalRecord{
		RunID:                "RUN-PERSIST-01",
		Epoch:                100,
		BatchSeq:             1,
		DecisionID:           "DEC-P1",
		RuntimeVersion:       journal.CurrentRuntimeVersion,
		PolicyName:           "PiecewiseVFA",
		PolicyParamHash:      "param-vfa",
		InitialStateHash:     "state-vfa",
		ActionHash:           "action-vfa",
		NextStateHash:        "next-vfa",
		TotalNetContribution: 3200.0,
	}
	rec.Seal()

	if err := fs1.Append(rec); err != nil {
		t.Fatalf("FileStore.Append failed: %v", err)
	}

	// Re-open in a second instance
	fs2, err := journal.NewFileStore(filePath)
	if err != nil {
		t.Fatalf("second NewFileStore failed: %v", err)
	}

	loadedRec, err := fs2.Get("DEC-P1")
	if err != nil {
		t.Fatalf("failed retrieving record from re-opened store: %v", err)
	}
	if loadedRec.RecordHash != rec.RecordHash {
		t.Fatalf("hash mismatch after reload: %s vs %s", loadedRec.RecordHash, rec.RecordHash)
	}
}

func TestMemoryStore_ConcurrentAccessSafety(t *testing.T) {
	store := journal.NewMemoryStore()
	runID := "RUN-CONCURRENT"

	var wg sync.WaitGroup
	// Simulate concurrent reads while appending
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for step := 0; step < 10; step++ {
				_, _ = store.ListByRun(runID)
				_, _ = store.LastRecord(runID)
				_, _, _ = store.VerifyRunChain(runID)
			}
		}(i)
	}
	wg.Wait()
}
