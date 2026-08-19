// Package hos implements the high-fidelity Hours-of-Service (HOS) regulatory simulation engine
// for Project Mittens, replicating the complete capabilities of the Federal Motor Carrier Safety
// Administration (FMCSA) DOT regulations and the legacy Java hos-simulator.
//
// Ownership & Responsibility:
//   - Tracks and simulates driver duty status clocks (Hygiene, Driving, Shift, Rolling Cycle).
//   - Enforces US DOT regulations: 11-hour driving limit, 14-hour on-duty shift window,
//     30-minute mandatory rest break after 8 hours driving, 70-hour / 8-day rolling cycle,
//     34-hour restart, and 8/2 / 7/3 sleeper berth split provisions.
//   - Supports regional and fleet operational variations (US Solo, US Team, Canadian Solo, Canadian Team).
//   - Simulates operational event sequences (Drive, Loading, Unloading, Rest, Hold, Border Crossing)
//     with automatic insertion of statutory rest breaks and daily/weekly resets.
//   - Evaluates physical trip feasibility and earliest completion timelines for load dispatches.
//
// Dependency Boundaries:
//   - Imports: Standard library only (time, math, fmt, errors).
//   - Imported By: /internal/domain/model, /internal/domain/policy, /internal/service.
//   - Strict Rule: Zero I/O, pure deterministic arithmetic, no global mutable state.
//
// Active Invariants:
//   - Inviolate 0 (Explicit Config): All regulatory thresholds and duty rules are explicit structs.
//   - Inviolate 5 (Immutability): Driver clock updates and event simulations allocate and return
//     fresh state instances.
//   - Inviolate 6 (Lock-Free Concurrency): Zero mutexes; completely safe for parallel branch evaluation.
package hos
