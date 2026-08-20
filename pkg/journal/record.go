package journal

import (
	"fmt"
	"math"
)

// GenesisPrevHash represents the canonical 64-character zero hash used as PrevRecordHash for epoch 0 decisions.
const GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"

// CurrentRuntimeVersion defines the active engine specification version pin.
const CurrentRuntimeVersion = "mittens-v1.2.0"

// JournalRecord captures the full-fidelity mathematical state snapshot and cryptographic hashes
// of an atomic optimization decision step, enabling bit-exact replay and tamper-evident Merkle verification.
type JournalRecord struct {
	// Logical Coordinates (deterministic logical clock)
	RunID      string `json:"run_id"`
	Epoch      int64  `json:"epoch"`
	BatchSeq   int    `json:"batch_seq"`
	DecisionID string `json:"decision_id"`

	// Merkle Chaining
	PrevRecordHash string `json:"prev_record_hash"`
	RecordHash     string `json:"record_hash"`

	// Semantic Pins & Versioning
	RuntimeVersion  string `json:"runtime_version"`
	PolicyName      string `json:"policy_name"`
	PolicyParamHash string `json:"policy_param_hash"`

	// Initial State Snapshots & Hashes (S_t)
	InitialStateHash      string `json:"initial_state_hash"`
	ResourceStateBytes    []byte `json:"resource_state_bytes"`
	InformationStateBytes []byte `json:"information_state_bytes"`
	BeliefStateBytes      []byte `json:"belief_state_bytes"`

	// Decision Action (a_t)
	ActionHash   string `json:"action_hash"`
	ActionBytes  []byte `json:"action_bytes"`
	MatchedCount int    `json:"matched_count"`

	// Outcome & Economic Attribution
	EvaluatedArcsCount   int     `json:"evaluated_arcs_count"`
	TotalNetContribution float64 `json:"total_net_contribution"`

	// External Observation Signals (W_{t+1})
	RealizedObservationsBytes []byte `json:"realized_observations_bytes,omitempty"`

	// Post-Transition State Snapshots & Hashes (S_{t+1})
	NextStateHash             string `json:"next_state_hash"`
	NextResourceStateBytes    []byte `json:"next_resource_state_bytes,omitempty"`
	NextInformationStateBytes []byte `json:"next_information_state_bytes,omitempty"`
	NextBeliefStateBytes      []byte `json:"next_belief_state_bytes,omitempty"`
}

// ComputeRecordHash computes the canonical SHA-256 digest binding the previous record hash
// with the logical coordinates, semantic pins, input state, action, and resulting next state.
func (r *JournalRecord) ComputeRecordHash() string {
	contrib := r.TotalNetContribution
	if math.Abs(contrib) < 1e-9 {
		contrib = 0.0
	}
	payload := fmt.Sprintf(
		"%s|%s|%d|%d|%s|%s|%s|%s|%s|%s|%s|%.6f",
		r.PrevRecordHash,
		r.RunID,
		r.Epoch,
		r.BatchSeq,
		r.DecisionID,
		r.RuntimeVersion,
		r.PolicyName,
		r.PolicyParamHash,
		r.InitialStateHash,
		r.ActionHash,
		r.NextStateHash,
		contrib,
	)
	return ComputeSHA256([]byte(payload))
}

// Seal finalizes the record by computing and setting its RecordHash.
func (r *JournalRecord) Seal() {
	if r.PrevRecordHash == "" {
		r.PrevRecordHash = GenesisPrevHash
	}
	if r.RuntimeVersion == "" {
		r.RuntimeVersion = CurrentRuntimeVersion
	}
	r.RecordHash = r.ComputeRecordHash()
}

// VerifyIntegrity checks if the record's self-contained RecordHash is cryptographically valid.
func (r *JournalRecord) VerifyIntegrity() bool {
	return r.RecordHash != "" && r.RecordHash == r.ComputeRecordHash()
}
