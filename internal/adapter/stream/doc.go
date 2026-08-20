// Package stream implements high-throughput, non-blocking real-time streaming ingestion
// adapters for electronic logging device (ELD) telemetry and transportation management system (TMS) load tenders.
//
// Ownership & Responsibility:
//   - Ingests real-time GPS telemetry, hours-of-service (HOS) clock updates, and Projected Time of Availability (PTA) from ELD providers (e.g. Samsara, Motive).
//   - Ingests dynamic load tenders (EDI 204 / JSON) and tender cancellations from customer TMS platforms.
//   - Maintains thread-safe in-memory stream buffers and produces fresh, immutable domain ResourceState (R_t) instances for rolling optimization.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /internal/domain/model/hos, /pkg/logging, /pkg/telemetry
//   - Imported By: /internal/adapter/api, /cmd/server, /cmd/optimizer
//   - Strict Rule: Domain model and policy layers must never import this package.
//
// Active Invariants:
//   - Inviolate 0 (Explicit Config): All ingestion buffer thresholds and sync policies are configured via explicit structs.
//   - Inviolate 5 (Immutability): Stream synchronization always allocates and returns fresh ResourceState pointers rather than mutating in-place.
//   - Inviolate 6 (Concurrency): Stream buffer read snapshots utilize mutex-guarded defensive cloning alongside lock-free atomic telemetry counters.
package stream
