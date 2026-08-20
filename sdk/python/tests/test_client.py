"""Unit tests for MittensClient request dispatching and response handling."""

import unittest
from unittest.mock import patch, MagicMock
import json
import io
import sys
import os

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from mittens.client import MittensClient
from mittens.models import (
    LocationDTO,
    DriverDTO,
    LoadDTO,
    OptimizeResponse,
    ScenarioSummary,
    ScenarioDetail,
    ReplayScorecard,
    MerkleChainVerification,
)
from mittens.exceptions import MittensAPIError


class TestMittensClient(unittest.TestCase):
    def setUp(self):
        self.client = MittensClient(base_url="http://localhost:8080")

    @patch("urllib.request.urlopen")
    def test_list_scenarios(self, mock_urlopen):
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = json.dumps({
            "scenarios": [
                {
                    "id": "07_test_dispatch",
                    "name": "07_test_dispatch",
                    "description": "Baseline 3-driver 2-load scenario",
                    "category": "Dispatch Parity",
                    "driver_count": 3,
                    "load_count": 2,
                    "default_policy": "CFA",
                }
            ],
            "count": 1,
        }).encode("utf-8")
        mock_urlopen.return_value.__enter__.return_value = mock_response

        scenarios = self.client.list_scenarios()
        self.assertEqual(len(scenarios), 1)
        self.assertEqual(scenarios[0].id, "07_test_dispatch")
        self.assertEqual(scenarios[0].driver_count, 3)

    @patch("urllib.request.urlopen")
    def test_optimize(self, mock_urlopen):
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = json.dumps({
            "decision_id": "DEC-CFA_Parametric-1787251200-0001",
            "run_id": "RUN-CFA_Parametric",
            "epoch": 1787251200,
            "match_count": 1,
            "matches": [
                {
                    "driver_id": "DRV-01",
                    "load_id": "LOAD-01",
                    "dispatch_epoch": 1787251200,
                    "estimated_contribution": 850.50,
                }
            ],
            "total_net_contribution": 850.50,
            "execution_duration_ms": 0.42,
        }).encode("utf-8")
        mock_urlopen.return_value.__enter__.return_value = mock_response

        loc = LocationDTO(node_id="CHI", lat=41.8781, lon=-87.6298)
        drivers = [DriverDTO(id="DRV-01", current_location=loc, home_location=loc, available_epoch=1787251200)]
        loads = [LoadDTO(id="LOAD-01", origin=loc, destination=loc, pickup_earliest_epoch=1787251200, pickup_latest_epoch=1787265600, delivery_earliest_epoch=1787258400, delivery_latest_epoch=1787280000, revenue=1200.0)]

        res = self.client.optimize(drivers=drivers, loads=loads, policy_class="CFA", competitor_scale="N1")
        self.assertEqual(res.decision_id, "DEC-CFA_Parametric-1787251200-0001")
        self.assertEqual(res.match_count, 1)
        self.assertEqual(res.matches[0].driver_id, "DRV-01")
        self.assertEqual(res.total_net_contribution, 850.50)

    @patch("urllib.request.urlopen")
    def test_verify_merkle_chain(self, mock_urlopen):
        mock_response = MagicMock()
        mock_response.status = 200
        mock_response.read.return_value = json.dumps({
            "run_id": "RUN-CFA_Parametric",
            "is_valid": True,
            "latest_record_hash": "d8512927c4b4cce971b11b10efb44c32791cc6fc0012db01f3fb8c00d718ec56",
            "verification_message": "Chain continuous and cryptographic self-hashes verified",
        }).encode("utf-8")
        mock_urlopen.return_value.__enter__.return_value = mock_response

        chain = self.client.verify_merkle_chain("RUN-CFA_Parametric")
        self.assertTrue(chain.is_valid)
        self.assertEqual(chain.latest_record_hash, "d8512927c4b4cce971b11b10efb44c32791cc6fc0012db01f3fb8c00d718ec56")


if __name__ == "__main__":
    unittest.main()
