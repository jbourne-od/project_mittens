// Package rules implements a high-performance, Common Expression Language (CEL) based
// business rules engine for carrier optimization in Project Mittens.
//
// Ownership & Responsibility:
//   - Compiles declarative rule conditions and adjustment expressions using google/cel-go.
//   - Evaluates driver, load, and candidate arc contexts to apply dynamic bonuses, penalties,
//     mileage rate modifiers, and operational feasibility bans.
//   - Replicates the business logic capabilities of legacy SmartTlRuleRegistry.java.
//
// Dependency Boundaries:
//   - Imports: /internal/domain/model, /pkg/logging, github.com/google/cel-go
//   - Imported By: /internal/domain/policy, /internal/domain/model/feasibility, /internal/service
//   - Strict Rule: Zero I/O, zero database queries, and zero global mutable state.
//     All compiled CEL programs are immutable and safe for concurrent evaluation across lookahead trees.
package rules
