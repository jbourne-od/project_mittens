package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

var (
	// ErrRecordNotFound is returned when a requested decision ID does not exist.
	ErrRecordNotFound = errors.New("journal: record not found")
	// ErrCorruptedChain is returned when a Merkle chain verification detects a broken hash link.
	ErrCorruptedChain = errors.New("journal: corrupted cryptographic hash chain")
	// ErrDuplicateRecord is returned when attempting to append a record with an existing DecisionID.
	ErrDuplicateRecord = errors.New("journal: duplicate record for decision ID")
)

// JournalStore provides an append-only, tamper-evident repository for cryptographic journal records.
type JournalStore interface {
	// Append commits a new sealed JournalRecord to the store, extending the run's Merkle chain.
	Append(record JournalRecord) error
	// Get retrieves a record by its unique DecisionID.
	Get(decisionID string) (JournalRecord, error)
	// ListByRun returns all records for a given RunID in logical execution order.
	ListByRun(runID string) ([]JournalRecord, error)
	// LastRecord returns the latest record committed for a given RunID, or false if none exist.
	LastRecord(runID string) (JournalRecord, bool)
	// VerifyRunChain validates the cryptographic continuity of all records in a run's Merkle chain.
	VerifyRunChain(runID string) (bool, string, error)
}

// MemoryStore provides a high-efficiency in-memory JournalStore with atomic snapshot reading.
type MemoryStore struct {
	recordsMap atomic.Pointer[map[string]JournalRecord]
	runMap     atomic.Pointer[map[string][]JournalRecord]
	mu         sync.Mutex // Used only during append to prevent write races
}

// NewMemoryStore initializes an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{}
	emptyRecMap := make(map[string]JournalRecord)
	emptyRunMap := make(map[string][]JournalRecord)
	s.recordsMap.Store(&emptyRecMap)
	s.runMap.Store(&emptyRunMap)
	return s
}

// Append commits a new sealed JournalRecord into the MemoryStore.
func (s *MemoryStore) Append(rec JournalRecord) error {
	if !rec.VerifyIntegrity() {
		return fmt.Errorf("%w: record %s failed self-integrity hash check", ErrCorruptedChain, rec.DecisionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	curRecMap := *s.recordsMap.Load()
	if _, exists := curRecMap[rec.DecisionID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateRecord, rec.DecisionID)
	}

	curRunMap := *s.runMap.Load()
	runRecs := curRunMap[rec.RunID]

	// Verify Merkle link against previous record in this run
	if len(runRecs) == 0 {
		if rec.PrevRecordHash != GenesisPrevHash {
			return fmt.Errorf("%w: genesis record %s must have genesis prev hash, got %s", ErrCorruptedChain, rec.DecisionID, rec.PrevRecordHash)
		}
	} else {
		lastRec := runRecs[len(runRecs)-1]
		if rec.PrevRecordHash != lastRec.RecordHash {
			return fmt.Errorf("%w: record %s prev hash %s does not match previous record %s hash %s",
				ErrCorruptedChain, rec.DecisionID, rec.PrevRecordHash, lastRec.DecisionID, lastRec.RecordHash)
		}
	}

	// Copy-on-write update
	newRecMap := make(map[string]JournalRecord, len(curRecMap)+1)
	for k, v := range curRecMap {
		newRecMap[k] = v
	}
	newRecMap[rec.DecisionID] = rec

	newRunMap := make(map[string][]JournalRecord, len(curRunMap)+1)
	for k, v := range curRunMap {
		copied := make([]JournalRecord, len(v))
		copy(copied, v)
		newRunMap[k] = copied
	}
	newRunRecs := append(newRunMap[rec.RunID], rec)
	newRunMap[rec.RunID] = newRunRecs

	s.recordsMap.Store(&newRecMap)
	s.runMap.Store(&newRunMap)
	return nil
}

// Get retrieves a record by its unique DecisionID.
func (s *MemoryStore) Get(decisionID string) (JournalRecord, error) {
	recMap := *s.recordsMap.Load()
	rec, exists := recMap[decisionID]
	if !exists {
		return JournalRecord{}, fmt.Errorf("%w: %s", ErrRecordNotFound, decisionID)
	}
	return rec, nil
}

// ListByRun returns all records for a given RunID in logical execution order.
func (s *MemoryStore) ListByRun(runID string) ([]JournalRecord, error) {
	runMap := *s.runMap.Load()
	recs, exists := runMap[runID]
	if !exists {
		return []JournalRecord{}, nil
	}
	out := make([]JournalRecord, len(recs))
	copy(out, recs)
	return out, nil
}

// LastRecord returns the latest record committed for a given RunID.
func (s *MemoryStore) LastRecord(runID string) (JournalRecord, bool) {
	runMap := *s.runMap.Load()
	recs, exists := runMap[runID]
	if !exists || len(recs) == 0 {
		return JournalRecord{}, false
	}
	return recs[len(recs)-1], true
}

// VerifyRunChain validates the full cryptographic hash chain for a run.
func (s *MemoryStore) VerifyRunChain(runID string) (bool, string, error) {
	recs, err := s.ListByRun(runID)
	if err != nil {
		return false, "", err
	}
	if len(recs) == 0 {
		return true, "empty_run", nil
	}

	for i, r := range recs {
		if !r.VerifyIntegrity() {
			return false, r.DecisionID, fmt.Errorf("%w: record %s self-hash mismatch", ErrCorruptedChain, r.DecisionID)
		}
		if i == 0 {
			if r.PrevRecordHash != GenesisPrevHash {
				return false, r.DecisionID, fmt.Errorf("%w: genesis record %s has invalid prev hash", ErrCorruptedChain, r.DecisionID)
			}
		} else {
			prev := recs[i-1]
			if r.PrevRecordHash != prev.RecordHash {
				return false, r.DecisionID, fmt.Errorf("%w: link broken between %s and %s", ErrCorruptedChain, prev.DecisionID, r.DecisionID)
			}
		}
	}
	return true, recs[len(recs)-1].RecordHash, nil
}

// FileStore wraps a persistent JSON Lines file with an in-memory index for fast lookups.
type FileStore struct {
	filePath string
	mem      *MemoryStore
	mu       sync.Mutex
}

// NewFileStore opens or creates a JSON Lines journal file.
func NewFileStore(filePath string) (*FileStore, error) {
	mem := NewMemoryStore()
	fs := &FileStore{
		filePath: filePath,
		mem:      mem,
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := fs.loadFromFile(); err != nil {
			return nil, fmt.Errorf("journal: failed loading existing journal file: %w", err)
		}
	}

	return fs, nil
}

func (fs *FileStore) loadFromFile() error {
	f, err := os.Open(fs.filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow large lines (up to 16MB) for heavy state payloads
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec JournalRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("malformed journal JSON line: %w", err)
		}
		if err := fs.mem.Append(rec); err != nil {
			return fmt.Errorf("invalid journal record in file: %w", err)
		}
	}
	return scanner.Err()
}

// Append persists the record to the append-only file and updates the in-memory index.
func (fs *FileStore) Append(rec JournalRecord) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.mem.Append(rec); err != nil {
		return err
	}

	f, err := os.OpenFile(fs.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("journal: failed opening file for append: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("journal: failed marshaling record: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("journal: failed writing to file: %w", err)
	}

	return f.Sync()
}

// Get retrieves a record by its unique DecisionID.
func (fs *FileStore) Get(decisionID string) (JournalRecord, error) {
	return fs.mem.Get(decisionID)
}

// ListByRun returns all records for a given RunID.
func (fs *FileStore) ListByRun(runID string) ([]JournalRecord, error) {
	return fs.mem.ListByRun(runID)
}

// LastRecord returns the latest record committed for a given RunID.
func (fs *FileStore) LastRecord(runID string) (JournalRecord, bool) {
	return fs.mem.LastRecord(runID)
}

// VerifyRunChain validates the cryptographic continuity of all records in a run.
func (fs *FileStore) VerifyRunChain(runID string) (bool, string, error) {
	return fs.mem.VerifyRunChain(runID)
}
