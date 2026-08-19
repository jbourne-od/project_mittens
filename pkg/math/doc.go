// Package math provides pure, reusable mathematical kernels for optimization,
// linear algebra, and stochastic approximation.
//
// Ownership & Responsibility:
//   - Implements Simultaneous Perturbation Stochastic Approximation (SPSA) for offline
//     policy parameter optimization.
//   - Implements linear algebra routines (Cholesky decomposition, matrix operations).
//   - Implements correlated Knowledge Gradient (KG) calculations and belief simplex geometry.
//
// Dependency Boundaries:
//   - Imports: Standard library only.
//   - Imported By: /internal/domain/model, /internal/domain/policy, /internal/service.
//   - Strict Rule: Pure standalone mathematical library with zero dependencies on application
//     domain models or external frameworks.
//
// Active Invariants:
//   - Absolute Mathematical Rigor: Invariants, transitions, and probability matrices must be
//     exact and verifiable.
//   - Precision & Drift Checks: Floating-point routines must maintain numerical stability
//     and precision bounds (e.g. simplex sum to 1.0 \pm 1e-9).
package math
