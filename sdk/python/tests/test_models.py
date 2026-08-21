"""Unit tests for Python SDK dataclasses and serialization using stdlib unittest."""

import unittest
import sys
import os

# Add sdk/python to path for testing
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from mittens.models import (
    LocationDTO,
    DriverDTO,
    LoadDTO,
    EquipmentDTO,
    CostConfigDTO,
    FeasibilityConfigDTO,
    MatchDTO,
    OptimizeResponse,
    ScenarioSummary,
    ScenarioDetail,
    DecisionExplanation,
    ReplayScorecard,
    MerkleChainVerification,
    RepositionPlanResponse,
    _clean_dict,
)


class TestModels(unittest.TestCase):
    def test_location_serialization(self):
        loc = LocationDTO(node_id="CHI", lat=41.8781, lon=-87.6298)
        d = loc.to_dict()
        self.assertEqual(d, {"node_id": "CHI", "lat": 41.8781, "lon": -87.6298})

        loc_back = LocationDTO.from_dict(d)
        self.assertEqual(loc_back, loc)

    def test_driver_serialization(self):
        loc = LocationDTO(node_id="DET", lat=42.3314, lon=-83.0458)
        drv = DriverDTO(
            id="DRV-01",
            current_location=loc,
            home_location=loc,
            available_epoch=1787251200,
            drive_hours_remaining=10.5,
            duty_hours_remaining=13.0,
            equipment=EquipmentDTO(type="REEFER"),
        )
        d = drv.to_dict()
        self.assertEqual(d["id"], "DRV-01")
        self.assertEqual(d["equipment"]["type"], "REEFER")

        drv_back = DriverDTO.from_dict(d)
        self.assertEqual(drv_back.id, drv.id)
        self.assertEqual(drv_back.current_location.node_id, "DET")
        self.assertEqual(drv_back.equipment.type, "REEFER")

    def test_clean_dict(self):
        raw = {"a": 1, "b": None, "nested": {"c": 2, "d": None}, "arr": [{"e": 3, "f": None}]}
        cleaned = _clean_dict(raw)
        self.assertEqual(cleaned, {"a": 1, "nested": {"c": 2}, "arr": [{"e": 3}]})

    def test_replay_scorecard_parsing(self):
        raw = {
            "decision_id": "DEC-CFA_Parametric-1787251200-0001",
            "policy_class": "CFA",
            "status": "BIT_EXACT_MATCH",
            "drift_amount": 0.0,
            "initial_state_hash_match": True,
            "action_hash_match": True,
            "recorded_total_net": 1598.80,
            "replayed_total_net": 1598.80,
            "recorded_action_hash": "d8512927c4b4cce971b11b10efb44c32791cc6fc0012db01f3fb8c00d718ec56",
            "replayed_action_hash": "d8512927c4b4cce971b11b10efb44c32791cc6fc0012db01f3fb8c00d718ec56",
            "replay_duration_ms": 0.35,
        }
        scorecard = ReplayScorecard.from_dict(raw)
        self.assertEqual(scorecard.status, "BIT_EXACT_MATCH")
        self.assertEqual(scorecard.drift_amount, 0.0)
        self.assertTrue(scorecard.action_hash_match)


if __name__ == "__main__":
    unittest.main()
