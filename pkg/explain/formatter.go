package explain

import (
	"fmt"
	"strings"
)

// Formatter renders DecisionExplanation models into structured Markdown and human-readable text.
type Formatter struct{}

// NewFormatter initializes a new Formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// FormatMarkdown generates a comprehensive, GitHub-Flavored Markdown report suitable for dispatch modals and UI views.
func (f *Formatter) FormatMarkdown(exp *DecisionExplanation) string {
	if exp == nil {
		return "# Decision Explanation\n\n*No explanation data provided.*"
	}

	var b strings.Builder

	// 1. Header & Executive Summary
	b.WriteString(fmt.Sprintf("# Optimization Decision Explanation: `%s`\n\n", exp.DecisionID))
	b.WriteString(fmt.Sprintf("> [!NOTE]\n> **Executive Summary:** %s\n\n", exp.ExecutiveSummary))

	// Key Metrics Summary Table
	b.WriteString("### 📊 Operational Overview\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("| :--- | :--- |\n")
	b.WriteString(fmt.Sprintf("| **Optimization Run ID** | `%s` |\n", exp.OptimizationRunID))
	b.WriteString(fmt.Sprintf("| **Policy Formulation** | `%s` |\n", exp.PolicyName))
	b.WriteString(fmt.Sprintf("| **Decision Epoch** | `%d` |\n", exp.BatchEpoch))
	b.WriteString(fmt.Sprintf("| **Total Fleet Drivers** | `%d` |\n", exp.TotalDrivers))
	b.WriteString(fmt.Sprintf("| **Active Matches Assigned** | **`%d`** |\n", exp.MatchedDriversCount))
	b.WriteString(fmt.Sprintf("| **Drivers Held Idle** | `%d` |\n", exp.IdleDriversCount))
	b.WriteString(fmt.Sprintf("| **Total Net Contribution** | **`$%.2f`** |\n", exp.TotalNetContribution))
	b.WriteString(fmt.Sprintf("| **Total Objective Score** | `$%.2f` |\n\n", exp.TotalObjectiveValue))

	// 2. Active Driver Matches Table
	if len(exp.MatchedExplanations) > 0 {
		b.WriteString("### 🚛 Assigned Driver Matches\n\n")
		b.WriteString("| Driver | Assigned Load | Score | Net Margin | Deadhead | Region | Top Rejected Alternative |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
		for _, m := range exp.MatchedExplanations {
			topAltStr := "*(None evaluated)*"
			if len(m.RejectedAlternatives) > 0 {
				top := m.RejectedAlternatives[0]
				topAltStr = fmt.Sprintf("`%s` (Δ -$%.2f)", top.LoadID, top.ScoreDelta)
			}
			b.WriteString(fmt.Sprintf(
				"| **`%s`** | **`%s`** | `$%.2f` | **`$%.2f`** | `%.1f mi` | `%s` | %s |\n",
				m.DriverID, m.AssignedLoadID, m.WinningScore, m.ImmediateNetMargin, m.DeadheadMiles, m.PostDecisionRegion, topAltStr,
			))
		}
		b.WriteString("\n")
	}

	// 3. Idle Drivers Table
	if len(exp.IdleExplanations) > 0 {
		b.WriteString("### 🛑 Idle Drivers & Capital Preservation\n\n")
		b.WriteString("| Driver | Reason Code | Candidates Evaluated | Best Available Candidate | Economic Summary |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		for _, idle := range exp.IdleExplanations {
			bestCandStr := "*(None)*"
			if idle.BestCandidateAlternative != nil {
				bestCandStr = fmt.Sprintf("`%s` (Score: `$%.2f`)", idle.BestCandidateAlternative.LoadID, idle.BestCandidateAlternative.TotalScore)
			}
			b.WriteString(fmt.Sprintf(
				"| **`%s`** | `%s` | `%d` | %s | %s |\n",
				idle.DriverID, idle.ReasonCode, idle.EvaluatedCandidatesCount, bestCandStr, idle.Summary,
			))
		}
		b.WriteString("\n")
	}

	// 4. Detailed Counterfactual Cards per Match
	if len(exp.MatchedExplanations) > 0 {
		b.WriteString("### 🔍 Detailed Counterfactual & Economic Breakdowns\n\n")
		for _, m := range exp.MatchedExplanations {
			b.WriteString(fmt.Sprintf("#### Driver `%s` ➔ Load `%s`\n\n", m.DriverID, m.AssignedLoadID))
			b.WriteString(fmt.Sprintf("*%s*\n\n", m.Summary))

			// Factor Breakdown Table
			b.WriteString("| Economic Factor | Value ($) | Contribution Sub-Total |\n")
			b.WriteString("| :--- | :--- | :--- |\n")
			b.WriteString(fmt.Sprintf("| Gross Load Revenue | `+$%.2f` | Revenue |\n", m.EconomicBreakdown.GrossRevenue))
			b.WriteString(fmt.Sprintf("| Loaded Drive Cost | `-$%.2f` | Variable Operating |\n", m.EconomicBreakdown.LoadedDriveCost))
			b.WriteString(fmt.Sprintf("| Empty Deadhead Cost | `-$%.2f` | Repositioning (`%.1f mi`) |\n", m.EconomicBreakdown.EmptyDeadheadCost, m.DeadheadMiles))
			if m.EconomicBreakdown.EmptyToHomeCost > 0 {
				b.WriteString(fmt.Sprintf("| Domicile Reposition Cost | `-$%.2f` | Home Repositioning |\n", m.EconomicBreakdown.EmptyToHomeCost))
			}
			if m.EconomicBreakdown.InsertedDwellCost > 0 {
				b.WriteString(fmt.Sprintf("| Inserted Dwell Cost | `-$%.2f` | Facility Appointment Wait |\n", m.EconomicBreakdown.InsertedDwellCost))
			}
			b.WriteString(fmt.Sprintf("| **Immediate Net Margin** | **`$%.2f`** | **Net Realized Margin** |\n", m.ImmediateNetMargin))
			if m.EconomicBreakdown.DownstreamRegionalVFA != 0 {
				b.WriteString(fmt.Sprintf("| Downstream Regional Value (VFA) | `+$%.2f` | Post-Decision Region `%s` |\n", m.EconomicBreakdown.DownstreamRegionalVFA, m.PostDecisionRegion))
			}
			if m.EconomicBreakdown.CompetitorRiskPremium != 0 {
				b.WriteString(fmt.Sprintf("| Competitor Risk Adjustment (MOMDP) | `-$%.2f` | Market Posture Risk |\n", m.EconomicBreakdown.CompetitorRiskPremium))
			}
			b.WriteString(fmt.Sprintf("| **Total Mathematical Score** | **`$%.2f`** | **Policy Objective** |\n\n", m.WinningScore))

			// Counterfactual Alternatives Table
			if len(m.RejectedAlternatives) > 0 {
				b.WriteString("**Evaluated Rejected Load Alternatives:**\n\n")
				b.WriteString("| Candidate Load | Score Delta (Δ) | Candidate Score | Deadhead | Rejection Rationale |\n")
				b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
				for _, alt := range m.RejectedAlternatives {
					b.WriteString(fmt.Sprintf(
						"| `%s` | **`-$%.2f`** | `$%.2f` | `%.1f mi` | %s |\n",
						alt.LoadID, alt.ScoreDelta, alt.TotalScore, alt.DeadheadMiles, alt.RejectionReason,
					))
				}
				b.WriteString("\n")
			}
			b.WriteString("---\n\n")
		}
	}

	// 5. Competitive POMDP Belief Shift
	if exp.BeliefShift != nil {
		b.WriteString("### 🧠 Competitive POMDP Belief Shift\n\n")
		b.WriteString(fmt.Sprintf("> **Dominant Market Posture:** `%s` (`%s`)\n", exp.BeliefShift.DominantPosture, exp.BeliefShift.PostureShiftType))
		b.WriteString(fmt.Sprintf("> **Pricing Recommendation:** %s\n\n", exp.BeliefShift.PricingAction))

		b.WriteString("| Latent Market Posture | Prior Probability $b_t$ | Posterior Probability $b_{t+1}$ | Shift Direction |\n")
		b.WriteString("| :--- | :--- | :--- | :--- |\n")
		for k, post := range exp.BeliefShift.PosteriorBelief {
			prior := exp.BeliefShift.PriorBelief[k]
			arrow := "➡️ Stable"
			if post > prior+0.05 {
				arrow = "⬆️ Increased"
			} else if post < prior-0.05 {
				arrow = "⬇️ Decreased"
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%.3f` | `%.3f` | %s |\n", k, prior, post, arrow))
		}
		b.WriteString("\n")
	}

	return b.String()
}
