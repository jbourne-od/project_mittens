#!/usr/bin/env python3
"""Project Mittens Sidecar Shadow Proxy Demonstration.

Demonstrates Stage 1 zero-risk shadow adoption:
1. Wrapping a simulated legacy Java dispatcher with the ShadowProxy.
2. Returning primary dispatches immediately with zero physical operational risk.
3. Concurrently executing Project Mittens Go Engine (/api/v1/optimize).
4. Generating real-time parity and profit lift diff scorecards.
"""

import sys
import os
import time

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from mittens import MittensClient, ShadowProxy, ShadowMatch, ShadowDiffReport


def main():
    print("=" * 70)
    print("PROJECT MITTENS: Stage 1 Zero-Risk Sidecar Shadow Proxy")
    print("=" * 70)

    client = MittensClient(base_url="http://localhost:8080")

    # Initialize ShadowProxy with non-blocking async execution
    def on_diff_telemetry(report: ShadowDiffReport):
        print(f"\n[Telemetry Event] Epoch {report.epoch}:")
        print(f" - Primary Duration: {report.primary_duration_ms:.2f} ms | Shadow Duration: {report.shadow_duration_ms:.2f} ms")
        print(f" - Agreement Rate: {report.agreement_rate * 100:.1f}% ({report.agreement_match_count} agreed pairs)")
        print(f" - Profit Lift: ${report.net_contribution_delta:+,.2f} ({report.profit_lift_ratio * 100:+.2f}%)")
        print(f" - Contract Divergences: {report.contract_divergence_count}")

    proxy = ShadowProxy(
        client=client,
        policy_class="CFA",
        competitor_scale="N1",
        async_mode=False,  # Synchronous for immediate script output
        telemetry_callback=on_diff_telemetry,
    )

    try:
        scenario = client.get_scenario("07_test_dispatch")
    except Exception as e:
        print(f"\nNote: Project Mittens Go Engine not reachable at localhost:8080 ({e}).")
        print("To run live, start `go run cmd/server/main.go` and re-run this script.")
        return

    print(f"\nLoaded Golden Scenario: {scenario.name} ({len(scenario.drivers)} Drivers, {len(scenario.loads)} Loads)")

    # Simulate a legacy Java dispatcher that solves the epoch
    def legacy_java_dispatcher():
        time.sleep(0.05)  # Simulate 50ms legacy Java solver latency
        return [
            ShadowMatch(driver_id=scenario.drivers[0].id, load_id=scenario.loads[0].id, contribution=1200.0, is_contract=True),
        ]

    print("\nExecuting live dispatch under ShadowProxy...")
    t0 = time.perf_counter()
    live_dispatches = proxy.execute_and_shadow(
        epoch=1787251200,
        drivers=scenario.drivers,
        loads=scenario.loads,
        primary_dispatcher=legacy_java_dispatcher,
    )
    total_time = (time.perf_counter() - t0) * 1000.0

    print(f"\nAuthoritative Live Dispatch returned to operations in {total_time:.2f} ms:")
    for match in live_dispatches:
        print(f"  -> Driver {match.driver_id} assigned to Load {match.load_id} (Contrib: ${match.contribution:,.2f})")

    # Print rolling scorecard summary
    scorecard = proxy.scorecard.summary()
    print("\n" + "=" * 70)
    print("ROLLING SHADOW SCORECARD SUMMARY")
    print("=" * 70)
    for k, v in scorecard.items():
        if isinstance(v, float):
            print(f"  {k:30s}: {v:,.2f}")
        else:
            print(f"  {k:30s}: {v}")

    proxy.shutdown()


if __name__ == "__main__":
    main()
