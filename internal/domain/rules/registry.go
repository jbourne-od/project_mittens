package rules

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/cel-go/cel"
	"github.com/optimaldynamics/project-mittens/pkg/logging"
)

// CompiledRule holds a verified Rule definition along with its executable CEL program bytecode.
type CompiledRule struct {
	Rule             Rule
	ConditionProgram cel.Program
	ValueProgram     cel.Program
}

// RuleRegistry manages and evaluates compiled CEL business rules over candidate dispatch contexts.
//
// In accordance with Inviolate 5 (Immutability) and Inviolate 6 (Lock-Free Hot Paths),
// RuleRegistry is completely stateless during evaluation and safe for parallel execution across goroutines.
type RuleRegistry struct {
	env    *cel.Env
	rules  []CompiledRule
	logger *slog.Logger
}

// NewRuleRegistry initializes a CEL evaluation environment, compiles all declarative rules,
// and returns an immutable RuleRegistry instance.
func NewRuleRegistry(rules []Rule, logger *slog.Logger) (*RuleRegistry, error) {
	if logger == nil {
		logger = logging.NewNop()
	}

	// 1. Declare CEL context schema
	env, err := cel.NewEnv(
		cel.Variable("driver", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("load", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("arc", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("rules: failed to initialize CEL environment: %w", err)
	}

	compiledRules := make([]CompiledRule, len(rules))

	// 2. Compile each declarative rule
	for i, r := range rules {
		if r.ConditionCEL == "" {
			return nil, fmt.Errorf("rules: rule %s has empty ConditionCEL", r.ID)
		}

		// Compile Condition Expression
		condAST, issues := env.Compile(r.ConditionCEL)
		if issues != nil && issues.Err() != nil {
			return nil, fmt.Errorf("rules: compile condition failed for rule %s: %w", r.ID, issues.Err())
		}
		if condAST.OutputType() != cel.BoolType {
			return nil, fmt.Errorf("rules: condition for rule %s must return bool, got %v", r.ID, condAST.OutputType())
		}

		condProg, err := env.Program(condAST)
		if err != nil {
			return nil, fmt.Errorf("rules: failed to build program for rule %s: %w", r.ID, err)
		}

		// Compile Optional Value Expression
		var valProg cel.Program
		if r.ValueCEL != "" {
			valAST, issues := env.Compile(r.ValueCEL)
			if issues != nil && issues.Err() != nil {
				return nil, fmt.Errorf("rules: compile value failed for rule %s: %w", r.ID, issues.Err())
			}
			valProg, err = env.Program(valAST)
			if err != nil {
				return nil, fmt.Errorf("rules: failed to build value program for rule %s: %w", r.ID, err)
			}
		}

		compiledRules[i] = CompiledRule{
			Rule:             r,
			ConditionProgram: condProg,
			ValueProgram:     valProg,
		}
	}

	return &RuleRegistry{
		env:    env,
		rules:  compiledRules,
		logger: logger,
	}, nil
}

// RuleCount returns the total number of compiled rules in the registry.
func (reg *RuleRegistry) RuleCount() int {
	if reg == nil {
		return 0
	}
	return len(reg.rules)
}

// Evaluate applies all compiled rules against the provided evaluation context map.
func (reg *RuleRegistry) Evaluate(ctx context.Context, evalCtx map[string]any) (RuleEvaluationResult, error) {
	result := NewRuleEvaluationResult()
	if reg == nil || len(reg.rules) == 0 {
		return result, nil
	}

	logger := logging.FromContext(ctx, reg.logger)

	for _, cr := range reg.rules {
		// Evaluate condition expression
		condVal, _, err := cr.ConditionProgram.Eval(evalCtx)
		if err != nil {
			logger.DebugContext(ctx, "rule condition evaluation failed",
				slog.String("rule_id", cr.Rule.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		matched, ok := condVal.Value().(bool)
		if !ok || !matched {
			continue
		}

		// Condition matched! Calculate rule adjustment value
		numVal := cr.Rule.StaticValue
		if cr.ValueProgram != nil {
			vVal, _, err := cr.ValueProgram.Eval(evalCtx)
			if err != nil {
				logger.DebugContext(ctx, "rule value evaluation failed",
					slog.String("rule_id", cr.Rule.ID),
					slog.String("error", err.Error()),
				)
				continue
			}
			switch v := vVal.Value().(type) {
			case float64:
				numVal = v
			case int64:
				numVal = float64(v)
			}
		}

		result.MatchedRuleIDs = append(result.MatchedRuleIDs, cr.Rule.ID)

		logger.DebugContext(ctx, "rule triggered",
			slog.String("rule_id", cr.Rule.ID),
			slog.String("target", string(cr.Rule.Target)),
			slog.String("operation", string(cr.Rule.Operation)),
			slog.Float64("value", numVal),
		)

		// Apply target adjustment
		switch cr.Rule.Target {
		case TargetBonus:
			switch cr.Rule.Operation {
			case OpAdd:
				result.Bonus += numVal
			case OpOverride:
				result.Bonus = numVal
			}

		case TargetInfeasible:
			result.IsInfeasible = true
			if cr.Rule.Message != "" {
				result.InfeasibilityReason = cr.Rule.Message
			} else {
				result.InfeasibilityReason = fmt.Sprintf("infeasible by rule %s", cr.Rule.ID)
			}

		case TargetLoadedRate:
			switch cr.Rule.Operation {
			case OpMultiply:
				result.LoadedRateMultiplier *= numVal
			case OpOverride:
				result.LoadedRateMultiplier = numVal
			}

		case TargetEmptyRate:
			switch cr.Rule.Operation {
			case OpMultiply:
				result.EmptyRateMultiplier *= numVal
			case OpOverride:
				result.EmptyRateMultiplier = numVal
			}

		case TargetEmptyToHome:
			switch cr.Rule.Operation {
			case OpMultiply:
				result.EmptyToHomeMultiplier *= numVal
			case OpOverride:
				result.EmptyToHomeMultiplier = numVal
			}

		case TargetFixedCost:
			switch cr.Rule.Operation {
			case OpMultiply:
				result.FixedCostMultiplier *= numVal
			case OpOverride:
				result.FixedCostMultiplier = numVal
			}

		case TargetMaxDeadhead:
			if numVal > 0 {
				result.MaxDeadheadOverride = numVal
			}
		}

		// Handle OpBan
		if cr.Rule.Operation == OpBan {
			result.IsInfeasible = true
			if result.InfeasibilityReason == "" {
				result.InfeasibilityReason = fmt.Sprintf("banned by rule %s", cr.Rule.ID)
			}
		}
	}

	return result, nil
}
