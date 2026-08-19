package service

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
)

// JournalEntry models an append-only, tamper-proof record of a single optimization epoch allocation.
//
// In accordance with Section 5.4 (Semantic Journaling) and Inviolate 7 (Complete Provenance),
// every dispatch decision captures full state summaries, evaluated alternatives, and policy coefficients.
type JournalEntry struct {
	DecisionID           string                    `json:"decision_id"`
	BatchEpoch           int64                     `json:"batch_epoch"`
	PolicyName           string                    `json:"policy_name"`
	MatchedCount         int                       `json:"matched_count"`
	TotalObjective       float64                   `json:"total_objective"`
	TotalNetContribution float64                   `json:"total_net_contribution"`
	Matches              []model.DriverLoadMatch   `json:"matches"`
	Provenance           policy.DecisionProvenance `json:"provenance"`
}

// Journal defines the interface for persisting optimization decision audit trails.
type Journal interface {
	Record(ctx context.Context, entry JournalEntry) error
	GetEntries() []JournalEntry
	Count() int
}

// MemoryJournal is an in-memory, thread-safe append-only Journal implementation.
type MemoryJournal struct {
	entries atomic.Pointer[[]JournalEntry]
}

// NewMemoryJournal initializes a new MemoryJournal.
func NewMemoryJournal() *MemoryJournal {
	j := &MemoryJournal{}
	initial := make([]JournalEntry, 0)
	j.entries.Store(&initial)
	return j
}

// Record appends a new JournalEntry immutably using atomic pointer swap.
func (j *MemoryJournal) Record(ctx context.Context, entry JournalEntry) error {
	for {
		oldPtr := j.entries.Load()
		oldSlice := *oldPtr
		newSlice := make([]JournalEntry, len(oldSlice)+1)
		copy(newSlice, oldSlice)
		newSlice[len(oldSlice)] = entry

		if j.entries.CompareAndSwap(oldPtr, &newSlice) {
			return nil
		}
	}
}

// GetEntries returns an immutable deep snapshot of all recorded journal entries.
func (j *MemoryJournal) GetEntries() []JournalEntry {
	ptr := j.entries.Load()
	if ptr == nil {
		return nil
	}
	oldSlice := *ptr
	out := make([]JournalEntry, len(oldSlice))
	copy(out, oldSlice)
	return out
}

// Count returns the total number of recorded journal entries.
func (j *MemoryJournal) Count() int {
	ptr := j.entries.Load()
	if ptr == nil {
		return 0
	}
	return len(*ptr)
}

// GenerateDecisionID creates a deterministic, reproducible decision identifier for an epoch and batch.
func GenerateDecisionID(policyName string, epoch int64, batchSeq int) string {
	return fmt.Sprintf("DEC-%s-%d-%04d", policyName, epoch, batchSeq)
}
