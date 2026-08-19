package model

import (
	"errors"
	"fmt"
)

var (
	// ErrNilStateComponent is returned when attempting to construct a State with a nil component.
	ErrNilStateComponent = errors.New("domain/model: state component cannot be nil")
)

// State represents the complete, factored Mixed-Observability MDP state variable at epoch t:
//
//	S^{ext}_t = (R_t, I_t, b_t) \in \mathcal{S} \times \mathcal{I} \times \Delta(\mathcal{H})
//
// Mathematical Formulation (HLD Section 6.1 & Inviolate 2):
//   - R_t \in \mathcal{S} is the fully observable physical fleet resource state (tractors, drivers, loads).
//   - I_t \in \mathcal{I} is the fully observable non-resource macro information state (fuel, weather, market indices).
//   - b_t \in \Delta(\mathcal{H}) is the recursive belief state distribution over latent competitor postures \Theta_t.
//
// Genericity (Inviolate 3):
// Parameterized by the competitor dimension type C (e.g. Monopolistic for N=0, AggregatedMarket for N=1).
//
// Immutability (Inviolate 5):
// State blocks cannot be mutated. Transitions allocate and return newly constructed *State[C] pointers.
type State[C CompetitorScale] struct {
	resource    *ResourceState
	information *InformationState
	belief      *Belief[C]
}

// NewState constructs and validates a factored State[C] instance.
// Returns an error if any component (resource, information, belief) is nil.
func NewState[C CompetitorScale](resource *ResourceState, info *InformationState, belief *Belief[C]) (*State[C], error) {
	if resource == nil {
		return nil, fmt.Errorf("%w: ResourceState is nil", ErrNilStateComponent)
	}
	if info == nil {
		return nil, fmt.Errorf("%w: InformationState is nil", ErrNilStateComponent)
	}
	if belief == nil {
		return nil, fmt.Errorf("%w: Belief is nil", ErrNilStateComponent)
	}

	return &State[C]{
		resource:    resource,
		information: info,
		belief:      belief,
	}, nil
}

// NewMonopolisticState provides a zero-overhead constructor for the degenerate N=0 monopolistic state.
//
// Automatically initializes the canonical Dirac delta belief state centered at \Theta_\emptyset (Inviolate 1).
func NewMonopolisticState(resource *ResourceState, info *InformationState) (*State[Monopolistic], error) {
	return NewState(resource, info, NewMonopolisticBelief())
}

// Resource returns the fully observable physical resource state (R_t).
func (s *State[C]) Resource() *ResourceState {
	return s.resource
}

// Information returns the macro information state (I_t).
func (s *State[C]) Information() *InformationState {
	return s.information
}

// Belief returns the recursive competitor belief state (b_t).
func (s *State[C]) Belief() *Belief[C] {
	return s.belief
}

// Clone returns an exact duplicate of the State block (Inviolate 5).
func (s *State[C]) Clone() *State[C] {
	return &State[C]{
		resource:    s.resource.Clone(),
		information: s.information.Clone(),
		belief:      s.belief.Clone(),
	}
}

// Transition computes the forward physical, macro-informational, and belief-state transition of the joint MOMDP state:
//
//	S_{t+1} = (R_{t+1}, I_{t+1}, b_{t+1}) = T(S_t, a_t, o_{t+1})
func (s *State[C]) Transition(action *Action, newLoads []Load) (*State[C], error) {
	if action == nil {
		action = NewAction(nil, nil)
	}

	nextRes, err := s.resource.Transition(action.Matches(), newLoads)
	if err != nil {
		return nil, fmt.Errorf("state: resource transition failed: %w", err)
	}

	return NewState(nextRes, s.information, s.belief)
}
