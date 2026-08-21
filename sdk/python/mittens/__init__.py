"""Project Mittens Python Client SDK.

A lightweight, zero-dependency Python interface for Project Mittens,
a high-efficiency POMDP carrier optimization and deterministic replay engine.
"""

from .client import MittensClient
from .models import (
    LocationDTO,
    EquipmentDTO,
    DriverDTO,
    LoadDTO,
    CostConfigDTO,
    FeasibilityConfigDTO,
    MatchDTO,
    OptimizeResponse,
    ScenarioSummary,
    ScenarioDetail,
    ValuationBreakdown,
    ArcExplanation,
    DecisionExplanation,
    ReplayScorecard,
    MerkleChainVerification,
    RepositionMoveDTO,
    RepositionPlanResponse,
)
from .shadow import (
    ShadowMatch,
    ShadowDiffReport,
    ShadowDiffEngine,
    RollingShadowScorecard,
    ShadowProxy,
)
from .exceptions import (
    MittensError,
    MittensAPIError,
    ReplayMismatchError,
    ChainIntegrityError,
)

__version__ = "1.0.0"
__all__ = [
    "MittensClient",
    "ShadowMatch",
    "ShadowDiffReport",
    "ShadowDiffEngine",
    "RollingShadowScorecard",
    "ShadowProxy",
    "LocationDTO",
    "EquipmentDTO",
    "DriverDTO",
    "LoadDTO",
    "CostConfigDTO",
    "FeasibilityConfigDTO",
    "MatchDTO",
    "OptimizeResponse",
    "ScenarioSummary",
    "ScenarioDetail",
    "ValuationBreakdown",
    "ArcExplanation",
    "DecisionExplanation",
    "ReplayScorecard",
    "MerkleChainVerification",
    "RepositionMoveDTO",
    "RepositionPlanResponse",
    "MittensError",
    "MittensAPIError",
    "ReplayMismatchError",
    "ChainIntegrityError",
]

