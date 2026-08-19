package rules

// ModificationTarget identifies the domain parameter or decision target modified by a rule.
// Replicates the legacy ModificationTarget enum from SmartTlRuleRegistry.java.
type ModificationTarget string

const (
	TargetBonus       ModificationTarget = "bonus"
	TargetInfeasible  ModificationTarget = "infeasible"
	TargetLoadedRate  ModificationTarget = "unitCostLoadedRate"
	TargetEmptyRate   ModificationTarget = "unitCostEmptyRate"
	TargetEmptyToHome ModificationTarget = "unitCostEmptyToHome"
	TargetFixedCost   ModificationTarget = "unitCostFixedPerLoad"
	TargetMaxDeadhead ModificationTarget = "maxEmptyDistance"
)

// Operation specifies the mathematical or logical mutation applied to the target parameter.
type Operation string

const (
	OpAdd      Operation = "ADD"      // target = target + value
	OpMultiply Operation = "MULTIPLY" // target = target * value
	OpOverride Operation = "OVERRIDE" // target = value
	OpBan      Operation = "BAN"      // target becomes physically infeasible
)

// Rule defines a declarative business rule evaluated against driver-load-arc contexts.
type Rule struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Target       ModificationTarget `json:"target"`
	Operation    Operation          `json:"operation"`
	ConditionCEL string             `json:"condition_cel"` // CEL expression returning bool
	ValueCEL     string             `json:"value_cel"`     // Optional CEL expression returning float64/string
	StaticValue  float64            `json:"static_value"`  // Static numeric value if ValueCEL is omitted
	Message      string             `json:"message"`       // Diagnostic reason if Target is TargetInfeasible
}

// RuleEvaluationResult captures the cumulative mutations resulting from rule evaluations over a candidate arc.
type RuleEvaluationResult struct {
	Bonus                 float64  `json:"bonus"`
	LoadedRateMultiplier  float64  `json:"loaded_rate_multiplier"`
	EmptyRateMultiplier   float64  `json:"empty_rate_multiplier"`
	EmptyToHomeMultiplier float64  `json:"empty_to_home_multiplier"`
	FixedCostMultiplier   float64  `json:"fixed_cost_multiplier"`
	MaxDeadheadOverride   float64  `json:"max_deadhead_override"`
	IsInfeasible          bool     `json:"is_infeasible"`
	InfeasibilityReason   string   `json:"infeasibility_reason"`
	MatchedRuleIDs        []string `json:"matched_rule_ids"`
}

// NewRuleEvaluationResult returns a clean evaluation result initialized with identity multipliers (1.0).
func NewRuleEvaluationResult() RuleEvaluationResult {
	return RuleEvaluationResult{
		Bonus:                 0.0,
		LoadedRateMultiplier:  1.0,
		EmptyRateMultiplier:   1.0,
		EmptyToHomeMultiplier: 1.0,
		FixedCostMultiplier:   1.0,
		MaxDeadheadOverride:   0.0,
		IsInfeasible:          false,
		InfeasibilityReason:   "",
		MatchedRuleIDs:        make([]string, 0),
	}
}
