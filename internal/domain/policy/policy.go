package policy

import (
	"context"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// Policy defines the universal contract for all Powell decision policies
// (PFAs, CFAs, VFAs, DLAs) over the factored MOMDP state space S_t = (R_t, I_t, b_t).
//
// In accordance with Inviolate 3 (Competitive Genericity) and Inviolate 4 (Closed Business Logic),
// all policies are parameterized by competitor scale C and defined statically at compile time.
type Policy[C model.CompetitorScale] interface {
	// Name returns the descriptive name of the policy (e.g. "CFA_Parametric", "VFA_Linear").
	Name() string

	// Evaluate evaluates the current state and returns an actionable decision (matching and pricing)
	// along with the complete decision provenance audit record.
	Evaluate(ctx context.Context, state *model.State[C]) (*model.Action, DecisionProvenance, error)
}
