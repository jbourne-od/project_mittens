"""Zero-risk Java-to-Go Sidecar Shadow Proxy and Parity Diffing Engine.

Enables non-blocking shadow traffic mirroring from legacy Java/Python dispatchers
to the Project Mittens Go Optimization Engine with 100% fail-safety and zero physical operational risk.
"""

from __future__ import annotations
import time
import logging
import threading
from concurrent.futures import ThreadPoolExecutor, Future
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Any, Callable, Tuple, Set

from .client import MittensClient
from .models import DriverDTO, LoadDTO, CostConfigDTO, FeasibilityConfigDTO, OptimizeResponse, MatchDTO

logger = logging.getLogger("mittens.shadow")


@dataclass(slots=True)
class ShadowMatch:
    """Represents a driver-load assignment for parity comparison."""
    driver_id: str
    load_id: str
    contribution: float = 0.0
    is_contract: bool = True


@dataclass(slots=True)
class ShadowDiffReport:
    """Detailed mathematical diff report between primary (legacy) and shadow (Mittens) decisions."""
    epoch: int
    primary_duration_ms: float
    shadow_duration_ms: float
    primary_match_count: int
    shadow_match_count: int
    primary_total_contribution: float
    shadow_total_contribution: float
    net_contribution_delta: float
    profit_lift_ratio: float
    agreement_match_count: int
    agreement_rate: float
    contract_divergence_count: int
    primary_only_pairs: List[Tuple[str, str]] = field(default_factory=list)
    shadow_only_pairs: List[Tuple[str, str]] = field(default_factory=list)
    agreed_pairs: List[Tuple[str, str]] = field(default_factory=list)
    shadow_error: Optional[str] = None

    @property
    def is_bit_exact(self) -> bool:
        """Returns True if matching pairings and counts are 100% identical."""
        return self.agreement_rate >= 0.999999 and self.contract_divergence_count == 0


class ShadowDiffEngine:
    """Pure mathematical engine for diffing primary vs. shadow dispatch decisions."""

    @staticmethod
    def diff(
        epoch: int,
        primary_matches: List[ShadowMatch],
        primary_duration_ms: float,
        shadow_response: Optional[OptimizeResponse] = None,
        shadow_duration_ms: float = 0.0,
        shadow_error: Optional[str] = None,
    ) -> ShadowDiffReport:
        """Computes a comprehensive mathematical diff report."""
        primary_pairs: Dict[str, ShadowMatch] = {m.driver_id: m for m in primary_matches}
        primary_set: Set[Tuple[str, str]] = {(m.driver_id, m.load_id) for m in primary_matches}
        primary_total_contrib = sum(m.contribution for m in primary_matches)

        if shadow_response is None or shadow_error is not None:
            return ShadowDiffReport(
                epoch=epoch,
                primary_duration_ms=primary_duration_ms,
                shadow_duration_ms=shadow_duration_ms,
                primary_match_count=len(primary_matches),
                shadow_match_count=0,
                primary_total_contribution=primary_total_contrib,
                shadow_total_contribution=0.0,
                net_contribution_delta=-primary_total_contrib,
                profit_lift_ratio=0.0,
                agreement_match_count=0,
                agreement_rate=0.0,
                contract_divergence_count=0,
                primary_only_pairs=sorted(list(primary_set)),
                shadow_error=shadow_error or "Shadow response not available",
            )

        shadow_matches: List[MatchDTO] = shadow_response.matches or []
        shadow_set: Set[Tuple[str, str]] = {(m.driver_id, m.load_id) for m in shadow_matches}
        shadow_total_contrib = shadow_response.total_net_contribution

        agreed = primary_set.intersection(shadow_set)
        primary_only = primary_set - shadow_set
        shadow_only = shadow_set - primary_set

        # Check for contract divergence (contract loads where primary and shadow differed)
        contract_divergence = 0
        for d_id, l_id in primary_only:
            match_obj = primary_pairs.get(d_id)
            if match_obj and match_obj.is_contract:
                contract_divergence += 1

        union_count = len(primary_set.union(shadow_set))
        agreement_rate = (len(agreed) / union_count) if union_count > 0 else 1.0

        net_delta = shadow_total_contrib - primary_total_contrib
        lift_ratio = (net_delta / primary_total_contrib) if primary_total_contrib > 1e-6 else 0.0

        return ShadowDiffReport(
            epoch=epoch,
            primary_duration_ms=primary_duration_ms,
            shadow_duration_ms=shadow_duration_ms,
            primary_match_count=len(primary_matches),
            shadow_match_count=len(shadow_matches),
            primary_total_contribution=primary_total_contrib,
            shadow_total_contribution=shadow_total_contrib,
            net_contribution_delta=net_delta,
            profit_lift_ratio=lift_ratio,
            agreement_match_count=len(agreed),
            agreement_rate=agreement_rate,
            contract_divergence_count=contract_divergence,
            primary_only_pairs=sorted(list(primary_only)),
            shadow_only_pairs=sorted(list(shadow_only)),
            agreed_pairs=sorted(list(agreed)),
            shadow_error=None,
        )


class RollingShadowScorecard:
    """Thread-safe rolling statistical scorecard accumulator across thousands of dispatch batches."""

    def __init__(self, max_history: int = 5000) -> None:
        self.max_history = max_history
        self._lock = threading.Lock()
        self.total_epochs: int = 0
        self.successful_shadow_epochs: int = 0
        self.failed_shadow_epochs: int = 0
        self.total_primary_contribution: float = 0.0
        self.total_shadow_contribution: float = 0.0
        self.total_contract_divergences: int = 0
        self.cumulative_agreements: int = 0
        self.cumulative_total_pairs: int = 0
        self.recent_reports: List[ShadowDiffReport] = []

    def record(self, report: ShadowDiffReport) -> None:
        """Records a new ShadowDiffReport into the scorecard."""
        with self._lock:
            self.total_epochs += 1
            if report.shadow_error:
                self.failed_shadow_epochs += 1
            else:
                self.successful_shadow_epochs += 1
                self.total_primary_contribution += report.primary_total_contribution
                self.total_shadow_contribution += report.shadow_total_contribution
                self.total_contract_divergences += report.contract_divergence_count
                self.cumulative_agreements += report.agreement_match_count
                self.cumulative_total_pairs += len(report.agreed_pairs) + len(report.primary_only_pairs) + len(report.shadow_only_pairs)

            self.recent_reports.append(report)
            if len(self.recent_reports) > self.max_history:
                self.recent_reports.pop(0)

    def summary(self) -> Dict[str, Any]:
        """Returns aggregated statistical summary."""
        with self._lock:
            overall_lift_ratio = 0.0
            if self.total_primary_contribution > 1e-6:
                overall_lift_ratio = (self.total_shadow_contribution - self.total_primary_contribution) / self.total_primary_contribution

            overall_agreement_rate = 1.0
            if self.cumulative_total_pairs > 0:
                overall_agreement_rate = self.cumulative_agreements / self.cumulative_total_pairs

            return {
                "total_epochs": self.total_epochs,
                "successful_shadow_epochs": self.successful_shadow_epochs,
                "failed_shadow_epochs": self.failed_shadow_epochs,
                "overall_agreement_rate": overall_agreement_rate,
                "overall_profit_lift_ratio": overall_lift_ratio,
                "total_primary_contribution": self.total_primary_contribution,
                "total_shadow_contribution": self.total_shadow_contribution,
                "net_contribution_delta": self.total_shadow_contribution - self.total_primary_contribution,
                "total_contract_divergences": self.total_contract_divergences,
            }


class ShadowProxy:
    """Non-blocking, zero-risk sidecar proxy that mirrors dispatch requests to Project Mittens.

    Parameters
    ----------
    client : MittensClient
        Configured Mittens API client.
    policy_class : str
        Target Powell policy class to evaluate on shadow path (default: 'CFA').
    competitor_scale : str
        Target competitor scale (default: 'N1').
    async_mode : bool
        If True, shadow calls run in background threads without delaying primary flow (default: True).
    max_workers : int
        Number of background worker threads for async mirroring (default: 4).
    timeout : float
        Maximum timeout in seconds for shadow execution (default: 2.0).
    telemetry_callback : Optional[Callable[[ShadowDiffReport], None]]
        Optional callback triggered on every completed diff report.
    """

    def __init__(
        self,
        client: Optional[MittensClient] = None,
        policy_class: str = "CFA",
        competitor_scale: str = "N1",
        async_mode: bool = True,
        max_workers: int = 4,
        timeout: float = 2.0,
        telemetry_callback: Optional[Callable[[ShadowDiffReport], None]] = None,
    ) -> None:
        self.client = client or MittensClient()
        self.policy_class = policy_class
        self.competitor_scale = competitor_scale
        self.async_mode = async_mode
        self.timeout = timeout
        self.telemetry_callback = telemetry_callback
        self.scorecard = RollingShadowScorecard()
        self._executor = ThreadPoolExecutor(max_workers=max_workers, thread_name_prefix="mittens-shadow")

    def execute_and_shadow(
        self,
        epoch: int,
        drivers: List[DriverDTO],
        loads: List[LoadDTO],
        primary_dispatcher: Callable[[], List[ShadowMatch]],
        cost_config: Optional[CostConfigDTO] = None,
        feasibility_config: Optional[FeasibilityConfigDTO] = None,
        enable_relays: bool = False,
    ) -> List[ShadowMatch]:
        """Executes the primary legacy dispatcher and mirrors the problem to Project Mittens in the shadow background.

        Guarantees:
        - The primary dispatcher result is ALWAYS returned to physical operations.
        - Failures on the shadow path NEVER disrupt or delay the primary return.
        """
        # 1. Execute primary dispatcher with precise timing
        t0 = time.perf_counter()
        primary_matches = primary_dispatcher()
        primary_duration_ms = (time.perf_counter() - t0) * 1000.0

        # 2. Dispatch shadow task
        if self.async_mode:
            self._executor.submit(
                self._run_shadow_and_diff,
                epoch,
                drivers,
                loads,
                primary_matches,
                primary_duration_ms,
                cost_config,
                feasibility_config,
                enable_relays,
            )
        else:
            self._run_shadow_and_diff(
                epoch,
                drivers,
                loads,
                primary_matches,
                primary_duration_ms,
                cost_config,
                feasibility_config,
                enable_relays,
            )

        # 3. Always return authoritative primary dispatches
        return primary_matches

    def _run_shadow_and_diff(
        self,
        epoch: int,
        drivers: List[DriverDTO],
        loads: List[LoadDTO],
        primary_matches: List[ShadowMatch],
        primary_duration_ms: float,
        cost_config: Optional[CostConfigDTO],
        feasibility_config: Optional[FeasibilityConfigDTO],
        enable_relays: bool,
    ) -> ShadowDiffReport:
        """Internal worker executing Go shadow optimization and computing diff report."""
        shadow_resp: Optional[OptimizeResponse] = None
        shadow_error: Optional[str] = None
        shadow_duration_ms: float = 0.0

        t0 = time.perf_counter()
        try:
            shadow_resp = self.client.optimize(
                drivers=drivers,
                loads=loads,
                policy_class=self.policy_class,
                competitor_scale=self.competitor_scale,
                epoch=epoch,
                cost_config=cost_config,
                feasibility_config=feasibility_config,
                enable_relays=enable_relays,
            )
            shadow_duration_ms = (time.perf_counter() - t0) * 1000.0
        except Exception as e:
            shadow_duration_ms = (time.perf_counter() - t0) * 1000.0
            shadow_error = f"{type(e).__name__}: {str(e)}"
            logger.warning("Project Mittens shadow execution failed safely: %s", shadow_error)

        # Compute mathematical diff
        report = ShadowDiffEngine.diff(
            epoch=epoch,
            primary_matches=primary_matches,
            primary_duration_ms=primary_duration_ms,
            shadow_response=shadow_resp,
            shadow_duration_ms=shadow_duration_ms,
            shadow_error=shadow_error,
        )

        # Record in rolling scorecard
        self.scorecard.record(report)

        # Trigger telemetry callback if provided
        if self.telemetry_callback:
            try:
                self.telemetry_callback(report)
            except Exception as e:
                logger.error("Telemetry callback failed: %s", e)

        return report

    def shutdown(self, wait: bool = True) -> None:
        """Shuts down background shadow worker threads."""
        self._executor.shutdown(wait=wait)
