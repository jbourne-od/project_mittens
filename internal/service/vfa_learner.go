// Package service implements carrier optimization orchestration, multi-day simulation,
// and adaptive value function approximation learning loops.
//
// In accordance with Project Mittens AGENTS.md:
//   - Dependency Boundaries: Imports /internal/domain/model, /internal/domain/policy, /pkg/math, /pkg/logging
//   - Inviolate 5: State immutability via value-based allocation.
//   - Inviolate 6: Lock-free concurrency on hot paths.
package service

import (
	"math"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/policy"
	pkgmath "github.com/optimaldynamics/project-mittens/pkg/math"
)

// VFALearningConfig specifies the learning hyperparameters for dual-subgradient VFA updates.
type VFALearningConfig struct {
	StepSize          float64 // Base learning step-size alpha (default: 0.1)
	HarmonicStepSize  bool    // If true, uses harmonic step-size alpha_t = a / (a + t - 1)
	HarmonicA         float64 // Parameter 'a' for harmonic step-size (default: 20.0)
	MaxSlopes         int     // Number of discrete piecewise slopes per region (default: 10)
	UseCKG            bool    // If true, also performs Correlated Knowledge Gradient Bayesian spatial updates
	CKGObservationVar float64 // Observation variance for CKG updates (default: 1.0)
}

// DefaultVFALearningConfig provides standard robust hyperparameters for VFA learning.
func DefaultVFALearningConfig() VFALearningConfig {
	return VFALearningConfig{
		StepSize:          0.1,
		HarmonicStepSize:  true,
		HarmonicA:         20.0,
		MaxSlopes:         10,
		UseCKG:            false,
		CKGObservationVar: 1.0,
	}
}

// PiecewiseVFALearner manages adaptive value function learning from dual marginal potentials (subgradients).
//
// In accordance with Powell (2022) / Topaloglu & Powell (2006):
//   - Receives optimal Dijkstra dual potentials u_d (driver shadow prices) after LAP solving.
//   - Updates post-decision marginal value slopes using CAVE (Concave Adaptive Value Estimation).
//   - Optionally generalizes subgradient feedback to geographically correlated neighboring regions via CKG.
type PiecewiseVFALearner struct {
	table          *policy.PiecewiseLinearVFATable
	ckg            *pkgmath.CorrelatedKnowledgeGradient
	regionManager  *model.RegionManager
	regionIndexMap map[string]int
	config         VFALearningConfig
	epochCount     int
}

// NewPiecewiseVFALearner constructs an immutable PiecewiseVFALearner.
func NewPiecewiseVFALearner(
	initialTable *policy.PiecewiseLinearVFATable,
	ckg *pkgmath.CorrelatedKnowledgeGradient,
	rm *model.RegionManager,
	regionIndexMap map[string]int,
	cfg VFALearningConfig,
) *PiecewiseVFALearner {
	if initialTable == nil {
		initialTable = policy.NewPiecewiseLinearVFATable(nil)
	}
	if rm == nil {
		rm = model.NewRegionManager(1.0, nil)
	}
	if cfg.StepSize <= 0 {
		cfg.StepSize = 0.1
	}
	if cfg.HarmonicA <= 0 {
		cfg.HarmonicA = 20.0
	}
	if cfg.MaxSlopes <= 0 {
		cfg.MaxSlopes = 10
	}

	copiedIndexMap := make(map[string]int, len(regionIndexMap))
	for k, v := range regionIndexMap {
		copiedIndexMap[k] = v
	}

	return &PiecewiseVFALearner{
		table:          initialTable,
		ckg:            ckg,
		regionManager:  rm,
		regionIndexMap: copiedIndexMap,
		config:         cfg,
		epochCount:     0,
	}
}

// Table returns the current PiecewiseLinearVFATable.
func (l *PiecewiseVFALearner) Table() *policy.PiecewiseLinearVFATable {
	return l.table
}

// CKG returns the current CorrelatedKnowledgeGradient model.
func (l *PiecewiseVFALearner) CKG() *pkgmath.CorrelatedKnowledgeGradient {
	if l.ckg == nil {
		return nil
	}
	return l.ckg.Clone()
}

// StepSize computes the active step-size for the current epoch.
func (l *PiecewiseVFALearner) StepSize() float64 {
	if !l.config.HarmonicStepSize {
		return l.config.StepSize
	}
	t := float64(l.epochCount + 1)
	return l.config.HarmonicA / (l.config.HarmonicA + t - 1.0)
}

// UpdateFromMatching processes the solved matching assignments and dual potentials from an optimization epoch,
// updating the PiecewiseLinearVFATable via CAVE and the CKG spatial covariance model.
//
// In accordance with Inviolate 5 (Immutability), returns a newly allocated *PiecewiseVFALearner.
func (l *PiecewiseVFALearner) UpdateFromMatching(
	sol policy.MatchingSolution,
	drivers []model.Driver,
	loads []model.Load,
) (*PiecewiseVFALearner, error) {
	driverMap := make(map[string]model.Driver, len(drivers))
	for _, d := range drivers {
		driverMap[d.ID] = d
	}

	loadMap := make(map[string]model.Load, len(loads))
	for _, load := range loads {
		loadMap[load.ID] = load
	}

	currentTable := l.table
	currentCKG := l.ckg
	stepSize := l.StepSize()

	// Track destination region resource additions
	regionArrivals := make(map[string]int)

	for _, match := range sol.Matches {
		load, okL := loadMap[match.LoadID]
		if !okL {
			continue
		}

		destRegion := l.regionManager.GetRegionID(load.Destination)
		arrivalIdx := regionArrivals[destRegion]
		regionArrivals[destRegion]++

		// Extract driver dual potential u_d (shadow price / opportunity cost)
		driverDual, okD := sol.DriverDualValues[match.DriverID]
		if !okD || math.IsNaN(driverDual) || math.IsInf(driverDual, 0) {
			continue
		}

		// Update piecewise slope at level arrivalIdx via CAVE level-clearing
		currentTable = currentTable.UpdateCAVE(
			destRegion,
			arrivalIdx,
			driverDual,
			stepSize,
			l.config.MaxSlopes,
		)

		// If CKG is enabled, update spatial Gaussian Process
		if l.config.UseCKG && currentCKG != nil {
			if rIdx, okR := l.regionIndexMap[destRegion]; okR {
				updatedCKG, err := currentCKG.UpdateBayesian(rIdx, driverDual, l.config.CKGObservationVar)
				if err == nil {
					currentCKG = updatedCKG
				}
			}
		}
	}

	return &PiecewiseVFALearner{
		table:          currentTable,
		ckg:            currentCKG,
		regionManager:  l.regionManager,
		regionIndexMap: l.regionIndexMap,
		config:         l.config,
		epochCount:     l.epochCount + 1,
	}, nil
}
