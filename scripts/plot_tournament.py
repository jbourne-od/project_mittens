# /// script
# dependencies = [
#   "matplotlib>=3.8.0",
#   "seaborn>=0.13.0",
#   "pandas>=2.1.0",
#   "scipy>=1.11.0",
# ]
# ///
"""
Project Mittens: Tournament Visual Analytics & Statistical Plotter
Generates publication-quality figures from tournament_results.json produced by the Go optimizer.
"""

import json
import os
import sys
import argparse

os.environ["MPLCONFIGDIR"] = os.path.join(os.getcwd(), "scratch", ".matplotlib_cache")
os.makedirs(os.environ["MPLCONFIGDIR"], exist_ok=True)

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import seaborn as sns
import pandas as pd
import numpy as np

def setup_style():
    sns.set_theme(style="whitegrid", font="sans-serif")
    plt.rcParams.update({
        "font.size": 11,
        "axes.labelsize": 12,
        "axes.titlesize": 13,
        "xtick.labelsize": 10,
        "ytick.labelsize": 10,
        "legend.fontsize": 11,
        "figure.titlesize": 15,
        "figure.dpi": 300,
    })

def load_results(json_path: str):
    if not os.path.exists(json_path):
        print(f"[ERROR] Tournament results file not found at: {json_path}")
        sys.exit(1)
    with open(json_path, "r") as f:
        return json.load(f)

def plot_tournament_analysis(data: dict, output_dir: str):
    os.makedirs(output_dir, exist_ok=True)
    setup_style()

    episodes = data["episodes"]
    df = pd.DataFrame(episodes)
    ttest = data["t_test"]

    # -------------------------------------------------------------
    # FIGURE 1: Net Contribution Distribution & Paired Delta Lift
    # -------------------------------------------------------------
    fig, axes = plt.subplots(1, 2, figsize=(15, 6))

    # Left: Boxplot / KDE of Net Contribution
    melted = df.melt(
        id_vars=["episode_index"],
        value_vars=["n0_net_contribution", "n1_net_contribution"],
        var_name="Policy",
        value_name="NetContribution"
    )
    melted["Policy"] = melted["Policy"].map({
        "n0_net_contribution": "N=0 Baseline (Myopic)",
        "n1_net_contribution": "N=1 MOMDP (Belief-Filtered)"
    })

    palette = {"N=0 Baseline (Myopic)": "#4C72B0", "N=1 MOMDP (Belief-Filtered)": "#2CA02C"}
    sns.boxplot(
        data=melted, x="Policy", y="NetContribution", hue="Policy",
        palette=palette, ax=axes[0], width=0.4, boxprops=dict(alpha=0.8), legend=False
    )
    sns.stripplot(
        data=melted, x="Policy", y="NetContribution", hue="Policy",
        palette=palette, ax=axes[0], size=7, jitter=0.15, alpha=0.9, edgecolor="black", linewidth=0.5, legend=False
    )
    axes[0].set_title("Net Contribution Distribution ($)", fontweight="bold")
    axes[0].set_ylabel("Net Contribution ($)")
    axes[0].set_xlabel("")
    axes[0].yaxis.set_major_formatter("${x:,.0f}")

    # Right: Per-Episode Lift Delta Bar Chart
    mean_diff = ttest.get("MeanDifference", ttest.get("mean_difference", 0.0))
    pct_lift = ttest.get("PercentLift", ttest.get("percent_lift", 0.0))
    p_val = ttest.get("PValueOneTailed", ttest.get("p_value_one_tailed", 0.0))
    t_stat = ttest.get("TStatistic", ttest.get("t_statistic", 0.0))
    mean_base = ttest.get("MeanBaseline", ttest.get("mean_baseline", 0.0))
    mean_cand = ttest.get("MeanCandidate", ttest.get("mean_candidate", 0.0))
    se = ttest.get("StdErrDifference", ttest.get("std_error", 1.0))

    colors = ["#2CA02C" if x >= 0 else "#D62728" for x in df["net_contribution_delta"]]
    bars = axes[1].bar(df["episode_index"], df["net_contribution_delta"], color=colors, alpha=0.85, edgecolor="black", linewidth=0.5)
    axes[1].axhline(0, color="black", linestyle="--", linewidth=1.0)
    axes[1].axhline(mean_diff, color="#1F77B4", linestyle="-.", linewidth=1.5, label=f"Mean Lift: +${mean_diff:,.2f}")
    axes[1].set_title("Episode-by-Episode Profit Lift (Δ = N1 - N0)", fontweight="bold")
    axes[1].set_xlabel("Episode Index")
    axes[1].set_ylabel("Net Contribution Delta ($)")
    axes[1].yaxis.set_major_formatter("${x:,.0f}")
    axes[1].legend(loc="upper left")

    plt.tight_layout()
    fig1_path = os.path.join(output_dir, "tournament_profit_distributions.png")
    plt.savefig(fig1_path, dpi=300)
    plt.close()
    print(f"[SUCCESS] Saved: {fig1_path}")

    # -------------------------------------------------------------
    # FIGURE 2: Statistical Significance & 95% Confidence Interval
    # -------------------------------------------------------------
    fig, axes = plt.subplots(1, 2, figsize=(15, 6))

    # Left: Mean Comparison with 95% Confidence Interval Error Bars
    means = [mean_base, mean_cand]
    labels = ["N=0 Baseline\n(Monopolistic)", "N=1 MOMDP\n(Belief-Filtered)"]
    x_pos = np.arange(len(labels))

    bars = axes[0].bar(x_pos, means, yerr=[1.96*se, 1.96*se], capsize=8, color=["#4C72B0", "#2CA02C"], alpha=0.85, edgecolor="black", width=0.45)
    axes[0].set_xticks(x_pos)
    axes[0].set_xticklabels(labels, fontweight="bold")
    axes[0].set_ylabel("Mean Net Contribution ($)")
    axes[0].set_title(f"Mean Net Contribution (95% CI)\np-value = {p_val:.4e} (t = {t_stat:.3f})", fontweight="bold")
    axes[0].yaxis.set_major_formatter("${x:,.0f}")

    # Annotate delta on plot
    axes[0].annotate(
        f"Lift: +${mean_diff:,.2f}\n(+{pct_lift:.2f}%)",
        xy=(0.5, max(means) * 0.5),
        xycoords="data",
        ha="center", fontsize=12, fontweight="bold",
        bbox=dict(boxstyle="round,pad=0.5", fc="#FFFFDD", ec="#888800")
    )

    # Right: Cumulative Net Profit Trajectory across episodes
    cum_n0 = df["n0_net_contribution"].cumsum()
    cum_n1 = df["n1_net_contribution"].cumsum()

    axes[1].plot(df["episode_index"], cum_n0, marker="o", label="N=0 Baseline (Cumulative)", color="#4C72B0", linewidth=2)
    axes[1].plot(df["episode_index"], cum_n1, marker="s", label="N=1 MOMDP (Cumulative)", color="#2CA02C", linewidth=2.5)
    axes[1].fill_between(df["episode_index"], cum_n0, cum_n1, where=(cum_n1 >= cum_n0), color="#2CA02C", alpha=0.2, label="MOMDP Value Surplus")
    axes[1].set_title("Cumulative Fleet Net Profit Across Tournament", fontweight="bold")
    axes[1].set_xlabel("Episode Index")
    axes[1].set_ylabel("Cumulative Profit ($)")
    axes[1].yaxis.set_major_formatter("${x:,.0f}")
    axes[1].legend(loc="upper left")

    plt.tight_layout()
    fig2_path = os.path.join(output_dir, "tournament_statistical_scorecard.png")
    plt.savefig(fig2_path, dpi=300)
    plt.close()
    print(f"[SUCCESS] Saved: {fig2_path}")

    # -------------------------------------------------------------
    # FIGURE 3: Win Rate & Freight Efficiency Tradeoff
    # -------------------------------------------------------------
    fig, axes = plt.subplots(1, 2, figsize=(15, 6))

    # Left: Win Rate Comparison
    df["n0_win_pct"] = df["n0_win_rate"] * 100.0
    df["n1_win_pct"] = df["n1_win_rate"] * 100.0
    win_melted = df.melt(
        id_vars=["episode_index"],
        value_vars=["n0_win_pct", "n1_win_pct"],
        var_name="Policy",
        value_name="WinRate"
    )
    win_melted["Policy"] = win_melted["Policy"].map({
        "n0_win_pct": "N=0 Baseline (Myopic)",
        "n1_win_pct": "N=1 MOMDP (Belief-Filtered)"
    })

    sns.violinplot(data=win_melted, x="Policy", y="WinRate", hue="Policy", palette=palette, ax=axes[0], inner="quartile", legend=False)
    axes[0].set_title("Market Auction Win Rate (%)", fontweight="bold")
    axes[0].set_ylabel("Win Rate (%)")
    axes[0].set_xlabel("")

    # Right: Revenue vs Cost Scatter by Policy
    axes[1].scatter(df["n0_operating_cost"], df["n0_gross_revenue"], color="#4C72B0", label="N=0 Baseline", alpha=0.8, s=60, edgecolors="black")
    axes[1].scatter(df["n1_operating_cost"], df["n1_gross_revenue"], color="#2CA02C", label="N=1 MOMDP", alpha=0.8, s=60, marker="s", edgecolors="black")

    # Add breakeven line
    all_costs = np.concatenate([df["n0_operating_cost"], df["n1_operating_cost"]])
    min_c, max_c = all_costs.min()*0.9, all_costs.max()*1.1
    axes[1].plot([min_c, max_c], [min_c, max_c], linestyle="--", color="gray", label="Breakeven (Rev = Cost)")

    axes[1].set_title("Gross Revenue vs Operating Cost (Efficiency Frontier)", fontweight="bold")
    axes[1].set_xlabel("Operating Cost ($)")
    axes[1].set_ylabel("Gross Revenue ($)")
    axes[1].xaxis.set_major_formatter("${x:,.0f}")
    axes[1].yaxis.set_major_formatter("${x:,.0f}")
    axes[1].legend(loc="upper left")

    plt.tight_layout()
    fig3_path = os.path.join(output_dir, "tournament_auction_efficiency.png")
    plt.savefig(fig3_path, dpi=300)
    plt.close()
    print(f"[SUCCESS] Saved: {fig3_path}")

def main():
    parser = argparse.ArgumentParser(description="Plot Project Mittens Tournament Results")
    parser.add_argument("--json", default="scratch/tournament_results.json", help="Path to tournament results JSON")
    parser.add_argument("--output-dir", default="docs/plots", help="Directory to save generated charts")
    args = parser.parse_args()

    data = load_results(args.json)
    plot_tournament_analysis(data, args.output_dir)
    print(f"\n[DONE] All tournament charts generated successfully in {args.output_dir}/")

if __name__ == "__main__":
    main()
