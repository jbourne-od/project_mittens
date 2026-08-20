#!/usr/bin/env python3
"""Project Mittens Python SDK Demonstration.

Shows how to connect to the Go optimization engine, load golden benchmark scenarios,
execute CFA/DLA competitive matching, inspect explainability waterfalls, and verify Merkle replay.
"""

import sys
import os

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from mittens import MittensClient


def main():
    client = MittensClient(base_url="http://localhost:8080")

    print("=== 1. Checking Go Optimization Engine Health ===")
    try:
        health = client.healthz()
        print(f"Engine Status: {health.get('status')}, Version: {health.get('version')}")
    except Exception as e:
        print(f"Note: Engine not running locally ({e}). Demonstration showing client interface.")
        return

    print("\n=== 2. Listing Authoritative Golden Benchmark Scenarios ===")
    scenarios = client.list_scenarios()
    for s in scenarios:
        print(f" - [{s.id}] {s.name} ({s.driver_count} Drivers, {s.load_count} Loads) -> {s.category}")

    target_id = "07_test_dispatch"
    print(f"\n=== 3. Loading Scenario State: {target_id} ===")
    scenario = client.get_scenario(target_id)
    print(f"Loaded {len(scenario.drivers)} drivers and {len(scenario.loads)} loads.")

    print("\n=== 4. Executing Single-Epoch CFA Optimization (N=1 Competitive POMDP) ===")
    res = client.optimize(
        drivers=scenario.drivers,
        loads=scenario.loads,
        policy_class="CFA",
        competitor_scale="N1",
    )
    print(f"Decision ID: {res.decision_id}")
    print(f"Matched Arcs: {res.match_count} / {len(scenario.loads)}")
    print(f"Total Net Contribution: ${res.total_net_contribution:,.2f}")
    print(f"Execution Duration: {res.execution_duration_ms:.2f} ms")

    for m in res.matches:
        print(f"  * Driver {m.driver_id} -> Load {m.load_id} (Est. Contribution: ${m.estimated_contribution:,.2f})")

    print(f"\n=== 5. Explaining Decision Provenance: {res.decision_id} ===")
    try:
        exp = client.explain_decision(res.decision_id)
        print(f"Evaluated Arcs: {exp.evaluated_arcs_count}")
        print(f"Attribution Summary:\n{exp.markdown_summary[:300]}...")
    except Exception as e:
        print(f"Explainability lookup note: {e}")

    print(f"\n=== 6. Deterministic Cryptographic Replay Audit ===")
    try:
        replay = client.replay_decision(res.decision_id)
        print(f"Replay Status: {replay.status}")
        print(f"Drift: {replay.drift_amount:.6f}")
        print(f"State Hash Match: {replay.initial_state_hash_match}")
        print(f"Action Hash Match: {replay.action_hash_match}")
    except Exception as e:
        print(f"Replay audit note: {e}")


if __name__ == "__main__":
    main()
