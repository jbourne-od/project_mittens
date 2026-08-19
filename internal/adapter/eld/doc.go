// Package eld adapts external electronic logging devices (ELD) and GPS telemetry
// feeds into domain resource states (R_t).
//
// Ownership & Responsibility:
//   - Ingests real-world driver hours-of-service (HOS) and vehicle location updates.
//   - Maps external provider schemas (e.g. Samsara, Motive) into immutable domain driver models.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/service
//   - Imported By: /cmd/optimizer
//   - Strict Rule: Domain model and policy packages must never import this package.
//
// Active Invariants:
//   - Inviolate 0 (Explicit Config): Connection credentials and telemetry polling intervals
//     must be explicitly injected via configuration structs.
//   - Inviolate 5 (Immutability): Ingested telemetry must produce newly allocated immutable
//     resource states rather than in-place mutations.
package eld
