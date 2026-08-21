"""Unit tests for ShadowProxy, ShadowDiffEngine, and RollingShadowScorecard."""

import unittest
from unittest.mock import MagicMock, patch
import time
import sys
import os

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from mittens.models import (
    LocationDTO,
    DriverDTO,
    LoadDTO,
    OptimizeResponse,
    MatchDTO,
)
from mittens.shadow import (
    ShadowMatch,
    ShadowDiffReport,
    ShadowDiffEngine,
    RollingShadowScorecard,
    ShadowProxy,
)
from mittens.exceptions import MittensAPIError, MittensError


class TestShadowDiffEngine(unittest.TestCase):
    def test_bit_exact_matches(self):
        primary_matches = [
            ShadowMatch(driver_id="D-01", load_id="L-01", contribution=500.0, is_contract=True),
            ShadowMatch(driver_id="D-02", load_id="L-02", contribution=800.0, is_contract=True),
        ]
        shadow_resp = OptimizeResponse(
            decision_id="DEC-01",
            run_id="RUN-01",
            epoch=1000,
            match_count=2,
            matches=[
                MatchDTO(driver_id="D-01", load_id="L-01", dispatch_epoch=1000, estimated_contribution=500.0),
                MatchDTO(driver_id="D-02", load_id="L-02", dispatch_epoch=1000, estimated_contribution=800.0),
            ],
            total_net_contribution=1300.0,
            execution_duration_ms=0.5,
        )

        report = ShadowDiffEngine.diff(
            epoch=1000,
            primary_matches=primary_matches,
            primary_duration_ms=120.0,
            shadow_response=shadow_resp,
            shadow_duration_ms=0.5,
        )

        self.assertTrue(report.is_bit_exact)
        self.assertEqual(report.agreement_rate, 1.0)
        self.assertEqual(report.contract_divergence_count, 0)
        self.assertEqual(report.net_contribution_delta, 0.0)
        self.assertEqual(report.profit_lift_ratio, 0.0)
        self.assertEqual(len(report.agreed_pairs), 2)
        self.assertEqual(len(report.primary_only_pairs), 0)
        self.assertEqual(len(report.shadow_only_pairs), 0)

    def test_competitive_profit_lift(self):
        primary_matches = [
            ShadowMatch(driver_id="D-01", load_id="L-01", contribution=500.0, is_contract=False),
        ]
        shadow_resp = OptimizeResponse(
            decision_id="DEC-02",
            run_id="RUN-02",
            epoch=2000,
            match_count=1,
            matches=[
                MatchDTO(driver_id="D-01", load_id="L-SPOT", dispatch_epoch=2000, estimated_contribution=800.0),
            ],
            total_net_contribution=800.0,
            execution_duration_ms=0.4,
        )

        report = ShadowDiffEngine.diff(
            epoch=2000,
            primary_matches=primary_matches,
            primary_duration_ms=150.0,
            shadow_response=shadow_resp,
            shadow_duration_ms=0.4,
        )

        self.assertFalse(report.is_bit_exact)
        self.assertEqual(report.agreement_rate, 0.0)
        self.assertEqual(report.contract_divergence_count, 0)  # L-01 is spot, so no contract divergence
        self.assertEqual(report.net_contribution_delta, 300.0)
        self.assertAlmostEqual(report.profit_lift_ratio, 0.60)  # +60% lift
        self.assertEqual(len(report.primary_only_pairs), 1)
        self.assertEqual(len(report.shadow_only_pairs), 1)

    def test_contract_divergence_flagged(self):
        primary_matches = [
            ShadowMatch(driver_id="D-01", load_id="L-CONTRACT", contribution=1000.0, is_contract=True),
        ]
        shadow_resp = OptimizeResponse(
            decision_id="DEC-03",
            run_id="RUN-03",
            epoch=3000,
            match_count=1,
            matches=[
                MatchDTO(driver_id="D-01", load_id="L-OTHER", dispatch_epoch=3000, estimated_contribution=1200.0),
            ],
            total_net_contribution=1200.0,
            execution_duration_ms=0.3,
        )

        report = ShadowDiffEngine.diff(
            epoch=3000,
            primary_matches=primary_matches,
            primary_duration_ms=90.0,
            shadow_response=shadow_resp,
            shadow_duration_ms=0.3,
        )

        self.assertFalse(report.is_bit_exact)
        self.assertEqual(report.contract_divergence_count, 1)

    def test_shadow_failure_handling(self):
        primary_matches = [
            ShadowMatch(driver_id="D-01", load_id="L-01", contribution=500.0, is_contract=True),
        ]

        report = ShadowDiffEngine.diff(
            epoch=4000,
            primary_matches=primary_matches,
            primary_duration_ms=80.0,
            shadow_response=None,
            shadow_error="Connection refused",
        )

        self.assertFalse(report.is_bit_exact)
        self.assertEqual(report.shadow_match_count, 0)
        self.assertEqual(report.shadow_error, "Connection refused")
        self.assertEqual(report.agreement_rate, 0.0)


class TestRollingShadowScorecard(unittest.TestCase):
    def test_scorecard_aggregation(self):
        scorecard = RollingShadowScorecard(max_history=100)

        # Record 2 successful bit-exact runs
        rep1 = ShadowDiffReport(
            epoch=1,
            primary_duration_ms=100.0,
            shadow_duration_ms=1.0,
            primary_match_count=2,
            shadow_match_count=2,
            primary_total_contribution=1000.0,
            shadow_total_contribution=1000.0,
            net_contribution_delta=0.0,
            profit_lift_ratio=0.0,
            agreement_match_count=2,
            agreement_rate=1.0,
            contract_divergence_count=0,
            agreed_pairs=[("D1", "L1"), ("D2", "L2")],
        )
        scorecard.record(rep1)

        rep2 = ShadowDiffReport(
            epoch=2,
            primary_duration_ms=100.0,
            shadow_duration_ms=1.0,
            primary_match_count=2,
            shadow_match_count=2,
            primary_total_contribution=1000.0,
            shadow_total_contribution=1300.0,
            net_contribution_delta=300.0,
            profit_lift_ratio=0.30,
            agreement_match_count=1,
            agreement_rate=0.333,
            contract_divergence_count=0,
            agreed_pairs=[("D1", "L1")],
            primary_only_pairs=[("D2", "L2")],
            shadow_only_pairs=[("D2", "L3")],
        )
        scorecard.record(rep2)

        # Record 1 failed shadow run
        rep3 = ShadowDiffReport(
            epoch=3,
            primary_duration_ms=100.0,
            shadow_duration_ms=0.0,
            primary_match_count=1,
            shadow_match_count=0,
            primary_total_contribution=500.0,
            shadow_total_contribution=0.0,
            net_contribution_delta=-500.0,
            profit_lift_ratio=0.0,
            agreement_match_count=0,
            agreement_rate=0.0,
            contract_divergence_count=0,
            shadow_error="Timeout",
        )
        scorecard.record(rep3)

        summary = scorecard.summary()
        self.assertEqual(summary["total_epochs"], 3)
        self.assertEqual(summary["successful_shadow_epochs"], 2)
        self.assertEqual(summary["failed_shadow_epochs"], 1)
        self.assertEqual(summary["total_primary_contribution"], 2000.0)
        self.assertEqual(summary["total_shadow_contribution"], 2300.0)
        self.assertEqual(summary["net_contribution_delta"], 300.0)
        self.assertAlmostEqual(summary["overall_profit_lift_ratio"], 0.15)


class TestShadowProxy(unittest.TestCase):
    def setUp(self):
        self.mock_client = MagicMock()
        self.proxy = ShadowProxy(
            client=self.mock_client,
            policy_class="CFA",
            competitor_scale="N1",
            async_mode=False,  # Synchronous for deterministic assertions
        )

    def test_fail_safe_execution_returns_primary_on_shadow_failure(self):
        """Assures that when Project Mittens throws an exception, the primary legacy dispatch is returned unmodified."""
        self.mock_client.optimize.side_effect = MittensError("Network connection reset")

        expected_primary = [
            ShadowMatch(driver_id="DRV-01", load_id="LOAD-01", contribution=600.0),
        ]

        def legacy_dispatcher():
            time.sleep(0.01)
            return expected_primary

        loc = LocationDTO(node_id="CHI", lat=41.87, lon=-87.62)
        drivers = [DriverDTO(id="DRV-01", current_location=loc, home_location=loc, available_epoch=1000)]
        loads = [LoadDTO(id="LOAD-01", origin=loc, destination=loc, pickup_earliest_epoch=1000, pickup_latest_epoch=2000, delivery_earliest_epoch=1500, delivery_latest_epoch=3000, revenue=1000.0)]

        # Must return primary matches without throwing
        result = self.proxy.execute_and_shadow(
            epoch=1000,
            drivers=drivers,
            loads=loads,
            primary_dispatcher=legacy_dispatcher,
        )

        self.assertEqual(result, expected_primary)
        self.assertEqual(self.proxy.scorecard.total_epochs, 1)
        self.assertEqual(self.proxy.scorecard.failed_shadow_epochs, 1)

    def test_telemetry_callback_triggered(self):
        """Verifies telemetry callback receives structured diff report."""
        telemetry_reports = []

        proxy = ShadowProxy(
            client=self.mock_client,
            async_mode=False,
            telemetry_callback=lambda r: telemetry_reports.append(r),
        )

        self.mock_client.optimize.return_value = OptimizeResponse(
            decision_id="DEC-TEST",
            run_id="RUN-TEST",
            epoch=1000,
            match_count=1,
            matches=[
                MatchDTO(driver_id="DRV-01", load_id="LOAD-01", dispatch_epoch=1000, estimated_contribution=600.0),
            ],
            total_net_contribution=600.0,
            execution_duration_ms=0.45,
        )

        def legacy_dispatcher():
            return [ShadowMatch(driver_id="DRV-01", load_id="LOAD-01", contribution=600.0)]

        loc = LocationDTO(node_id="CHI", lat=41.87, lon=-87.62)
        drivers = [DriverDTO(id="DRV-01", current_location=loc, home_location=loc, available_epoch=1000)]
        loads = [LoadDTO(id="LOAD-01", origin=loc, destination=loc, pickup_earliest_epoch=1000, pickup_latest_epoch=2000, delivery_earliest_epoch=1500, delivery_latest_epoch=3000, revenue=1000.0)]

        proxy.execute_and_shadow(
            epoch=1000,
            drivers=drivers,
            loads=loads,
            primary_dispatcher=legacy_dispatcher,
        )

        self.assertEqual(len(telemetry_reports), 1)
        self.assertTrue(telemetry_reports[0].is_bit_exact)
        self.assertEqual(telemetry_reports[0].agreement_rate, 1.0)


if __name__ == "__main__":
    unittest.main()
