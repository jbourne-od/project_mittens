"""Zero-dependency HTTP client for Project Mittens REST API."""

from __future__ import annotations
import json
import urllib.request
import urllib.error
from typing import List, Optional, Dict, Any, Union

from .models import (
    DriverDTO,
    LoadDTO,
    CostConfigDTO,
    FeasibilityConfigDTO,
    OptimizeResponse,
    ScenarioSummary,
    ScenarioDetail,
    DecisionExplanation,
    ReplayScorecard,
    MerkleChainVerification,
    RepositionPlanResponse,
    _clean_dict,
)
from .exceptions import MittensAPIError, MittensError


class MittensClient:
    """Type-safe, zero-dependency Python client for Project Mittens Go Optimization Engine.

    Parameters
    ----------
    base_url : str
        Base HTTP URL of the Mittens server (e.g. 'http://localhost:8080').
    timeout : float
        HTTP request timeout in seconds (default: 30.0).
    """

    def __init__(self, base_url: str = "http://localhost:8080", timeout: float = 30.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def _request(
        self,
        method: str,
        path: str,
        payload: Optional[Dict[str, Any]] = None,
    ) -> Any:
        url = f"{self.base_url}{path}"
        headers = {
            "User-Agent": "ProjectMittens-PythonSDK/1.0.0",
            "Accept": "application/json",
        }
        data = None
        if payload is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(_clean_dict(payload)).encode("utf-8")

        req = urllib.request.Request(url, data=data, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                status_code = resp.status
                body = resp.read().decode("utf-8")
                if not body:
                    return None
                return json.loads(body)
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8")
            error_code = "HTTP_ERROR"
            message = str(e)
            details = None
            try:
                err_json = json.loads(body)
                if isinstance(err_json, dict):
                    error_code = err_json.get("error_code", error_code)
                    message = err_json.get("message", message)
                    details = err_json.get("details")
            except Exception:
                message = body or str(e)
            raise MittensAPIError(e.code, error_code, message, details) from e
        except urllib.error.URLError as e:
            raise MittensError(f"Failed to connect to Mittens at {url}: {e.reason}") from e

    # -------------------------------------------------------------------------
    # Core Optimization & Simulation
    # -------------------------------------------------------------------------

    def optimize(
        self,
        drivers: List[DriverDTO],
        loads: List[LoadDTO],
        policy_class: str = "CFA",
        competitor_scale: str = "N1",
        epoch: Optional[int] = None,
        cost_config: Optional[CostConfigDTO] = None,
        feasibility_config: Optional[FeasibilityConfigDTO] = None,
        enable_relays: bool = False,
    ) -> OptimizeResponse:
        """Executes single-epoch optimal fleet matching via exact LAPJV."""
        req_payload: Dict[str, Any] = {
            "policy_class": policy_class,
            "competitor_scale": competitor_scale,
            "epoch": epoch,
            "enable_relays": enable_relays,
            "drivers": [d.to_dict() for d in drivers],
            "loads": [l.to_dict() for l in loads],
            "cost_config": cost_config.to_dict() if cost_config else None,
            "feasibility_config": feasibility_config.to_dict() if feasibility_config else None,
        }
        res = self._request("POST", "/api/v1/optimize", req_payload)
        return OptimizeResponse.from_dict(res)

    def simulate(
        self,
        drivers: List[DriverDTO],
        load_schedule: List[LoadDTO],
        start_epoch: int,
        horizon_days: int = 7,
        decision_step_hours: int = 24,
        policy_class: str = "CFA",
        competitor_scale: str = "N1",
        enable_relays: bool = True,
        min_relay_haul_miles: float = 350.0,
    ) -> Dict[str, Any]:
        """Runs rolling horizon continuous fleet simulation across multiple epochs."""
        req_payload = {
            "run_id": f"SIM-PY-{start_epoch}",
            "start_epoch": start_epoch,
            "horizon_days": horizon_days,
            "decision_step_hours": decision_step_hours,
            "policy_class": policy_class,
            "competitor_scale": competitor_scale,
            "enable_relays": enable_relays,
            "min_relay_haul_miles": min_relay_haul_miles,
            "drivers": [d.to_dict() for d in drivers],
            "load_schedule": [l.to_dict() for l in load_schedule],
        }
        return self._request("POST", "/api/v1/simulate", req_payload)

    # -------------------------------------------------------------------------
    # Scenario Catalog
    # -------------------------------------------------------------------------

    def list_scenarios(self) -> List[ScenarioSummary]:
        """Lists available golden parity datasets and operational carrier scenarios."""
        res = self._request("GET", "/api/v1/scenarios")
        return [ScenarioSummary.from_dict(s) for s in res.get("scenarios", [])]

    def get_scenario(self, scenario_id: str) -> ScenarioDetail:
        """Fetches full initial fleet and freight tender data for a scenario."""
        res = self._request("GET", f"/api/v1/scenarios/{scenario_id}")
        return ScenarioDetail.from_dict(res)

    # -------------------------------------------------------------------------
    # Explainability & Semantic Journal
    # -------------------------------------------------------------------------

    def list_decisions(self, limit: int = 50) -> List[Dict[str, Any]]:
        """Lists recent optimization decisions from the semantic journal."""
        return self._request("GET", f"/api/v1/decisions?limit={limit}")

    def get_decision(self, decision_id: str) -> Dict[str, Any]:
        """Retrieves raw provenance record for a specific decision."""
        return self._request("GET", f"/api/v1/decisions/{decision_id}")

    def explain_decision(self, decision_id: str) -> DecisionExplanation:
        """Generates full economic attribution waterfall and counterfactual ranking."""
        res = self._request("GET", f"/api/v1/decisions/{decision_id}/explain")
        return DecisionExplanation.from_dict(res)

    # -------------------------------------------------------------------------
    # Cryptographic Provenance & Deterministic Replay
    # -------------------------------------------------------------------------

    def replay_decision(self, decision_id: str) -> ReplayScorecard:
        """Executes offline deterministic state replay and verifies bit-exact state and action hashes."""
        res = self._request("POST", f"/api/v1/decisions/{decision_id}/replay")
        return ReplayScorecard.from_dict(res)

    def verify_merkle_chain(self, run_id: str) -> MerkleChainVerification:
        """Validates unbroken SHA-256 Merkle chain linking for an optimization run."""
        res = self._request("GET", f"/api/v1/journal/chain/{run_id}/verify")
        return MerkleChainVerification.from_dict(res)

    # -------------------------------------------------------------------------
    # Dynamic Fleet Repositioning
    # -------------------------------------------------------------------------

    def plan_reposition(
        self,
        drivers: List[DriverDTO],
        regional_targets: Optional[Dict[str, int]] = None,
        empty_mile_rate: float = 1.20,
        epoch: Optional[int] = None,
    ) -> RepositionPlanResponse:
        """Synthesizes empty tractor relocations using regional shadow price gradients."""
        req_payload = {
            "epoch": epoch,
            "drivers": [d.to_dict() for d in drivers],
            "regional_targets": regional_targets or {},
            "empty_mile_rate": empty_mile_rate,
        }
        res = self._request("POST", "/api/v1/reposition/plan", req_payload)
        return RepositionPlanResponse.from_dict(res)

    # -------------------------------------------------------------------------
    # System Health & Observability
    # -------------------------------------------------------------------------

    def healthz(self) -> Dict[str, Any]:
        """Checks optimization daemon health and database connectivity."""
        return self._request("GET", "/healthz")

    def get_metrics_raw(self) -> str:
        """Fetches raw Prometheus metrics scrape text."""
        url = f"{self.base_url}/metrics"
        req = urllib.request.Request(url, headers={"User-Agent": "ProjectMittens-PythonSDK/1.0.0"})
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            return resp.read().decode("utf-8")
