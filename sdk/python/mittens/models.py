"""Pure Python dataclass models for Project Mittens REST API."""

from __future__ import annotations
from dataclasses import dataclass, field, asdict
from typing import List, Dict, Optional, Any, Union


def _clean_dict(d: Dict[str, Any]) -> Dict[str, Any]:
    """Recursively remove None values from dictionary for clean JSON serialization."""
    clean = {}
    for k, v in d.items():
        if v is None:
            continue
        if isinstance(v, dict):
            clean[k] = _clean_dict(v)
        elif isinstance(v, list):
            clean[k] = [
                _clean_dict(item) if isinstance(item, dict) else item
                for item in v
            ]
        else:
            clean[k] = v
    return clean


@dataclass(slots=True)
class LocationDTO:
    """Geographic location node with latitude and longitude."""
    node_id: str
    lat: float
    lon: float

    def to_dict(self) -> Dict[str, Any]:
        return {"node_id": self.node_id, "lat": self.lat, "lon": self.lon}

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> LocationDTO:
        return cls(
            node_id=str(data.get("node_id", "")),
            lat=float(data.get("lat", 0.0)),
            lon=float(data.get("lon", 0.0)),
        )


@dataclass(slots=True)
class EquipmentDTO:
    """Trailer and equipment specifications."""
    type: str = "DRY_VAN"

    def to_dict(self) -> Dict[str, Any]:
        return {"type": self.type}

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> EquipmentDTO:
        return cls(type=str(data.get("type", "DRY_VAN")))


@dataclass(slots=True)
class DriverDTO:
    """Driver resource input representing physical capacity and HOS availability."""
    id: str
    current_location: LocationDTO
    home_location: LocationDTO
    available_epoch: int
    drive_hours_remaining: float = 11.0
    duty_hours_remaining: float = 14.0
    equipment: EquipmentDTO = field(default_factory=EquipmentDTO)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "id": self.id,
            "current_location": self.current_location.to_dict(),
            "home_location": self.home_location.to_dict(),
            "available_epoch": self.available_epoch,
            "drive_hours_remaining": self.drive_hours_remaining,
            "duty_hours_remaining": self.duty_hours_remaining,
            "equipment": self.equipment.to_dict(),
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> DriverDTO:
        return cls(
            id=str(data.get("id", "")),
            current_location=LocationDTO.from_dict(data.get("current_location", {})),
            home_location=LocationDTO.from_dict(data.get("home_location", {})),
            available_epoch=int(data.get("available_epoch", 0)),
            drive_hours_remaining=float(data.get("drive_hours_remaining", 11.0)),
            duty_hours_remaining=float(data.get("duty_hours_remaining", 14.0)),
            equipment=EquipmentDTO.from_dict(data.get("equipment", {})),
        )


@dataclass(slots=True)
class LoadDTO:
    """Freight shipment tender representing physical demand."""
    id: str
    origin: LocationDTO
    destination: LocationDTO
    pickup_earliest_epoch: int
    pickup_latest_epoch: int
    delivery_earliest_epoch: int
    delivery_latest_epoch: int
    revenue: float
    required_equipment: str = "DRY_VAN"

    def to_dict(self) -> Dict[str, Any]:
        return {
            "id": self.id,
            "origin": self.origin.to_dict(),
            "destination": self.destination.to_dict(),
            "pickup_earliest_epoch": self.pickup_earliest_epoch,
            "pickup_latest_epoch": self.pickup_latest_epoch,
            "delivery_earliest_epoch": self.delivery_earliest_epoch,
            "delivery_latest_epoch": self.delivery_latest_epoch,
            "revenue": self.revenue,
            "required_equipment": self.required_equipment,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> LoadDTO:
        return cls(
            id=str(data.get("id", "")),
            origin=LocationDTO.from_dict(data.get("origin", {})),
            destination=LocationDTO.from_dict(data.get("destination", {})),
            pickup_earliest_epoch=int(data.get("pickup_earliest_epoch", 0)),
            pickup_latest_epoch=int(data.get("pickup_latest_epoch", 0)),
            delivery_earliest_epoch=int(data.get("delivery_earliest_epoch", 0)),
            delivery_latest_epoch=int(data.get("delivery_latest_epoch", 0)),
            revenue=float(data.get("revenue", 0.0)),
            required_equipment=str(data.get("required_equipment", "DRY_VAN")),
        )


@dataclass(slots=True)
class CostConfigDTO:
    """Economic cost function parameterization."""
    fixed_cost_per_load: float = 50.0
    loaded_mile_rate: float = 1.50
    empty_mile_rate: float = 1.20
    empty_to_home_rate: float = 0.35
    early_arrival_per_hour: float = 20.0
    late_delivery_per_hour: float = 60.0
    driver_bonus_weight: float = 0.0

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> CostConfigDTO:
        return cls(**{k: v for k, v in data.items() if k in cls.__dataclass_fields__})


@dataclass(slots=True)
class FeasibilityConfigDTO:
    """Physical feasibility and HOS operational boundaries."""
    max_deadhead_miles: float = 350.0
    average_speed_mph: float = 50.0
    max_driving_hours_per_day: float = 11.0
    max_duty_hours_per_day: float = 14.0
    break_duration_hours: float = 0.5
    overnight_rest_hours: float = 10.0

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> FeasibilityConfigDTO:
        return cls(**{k: v for k, v in data.items() if k in cls.__dataclass_fields__})


@dataclass(slots=True)
class MatchDTO:
    """Single optimal driver-load assignment."""
    driver_id: str
    load_id: str
    dispatch_epoch: int
    estimated_contribution: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> MatchDTO:
        return cls(
            driver_id=str(data.get("driver_id", "")),
            load_id=str(data.get("load_id", "")),
            dispatch_epoch=int(data.get("dispatch_epoch", 0)),
            estimated_contribution=float(data.get("estimated_contribution", 0.0)),
        )


@dataclass(slots=True)
class OptimizeResponse:
    """Response returned by single-epoch optimal fleet matching."""
    decision_id: str
    run_id: str
    epoch: int
    match_count: int
    matches: List[MatchDTO]
    total_net_contribution: float
    execution_duration_ms: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> OptimizeResponse:
        return cls(
            decision_id=str(data.get("decision_id", "")),
            run_id=str(data.get("run_id", "")),
            epoch=int(data.get("epoch", 0)),
            match_count=int(data.get("match_count", 0)),
            matches=[MatchDTO.from_dict(m) for m in data.get("matches", [])],
            total_net_contribution=float(data.get("total_net_contribution", 0.0)),
            execution_duration_ms=float(data.get("execution_duration_ms", 0.0)),
        )


@dataclass(slots=True)
class ScenarioSummary:
    """Authoritative golden test scenario summary."""
    id: str
    name: str
    description: str
    category: str
    driver_count: int
    load_count: int
    default_policy: str = "CFA"

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ScenarioSummary:
        return cls(
            id=str(data.get("id", "")),
            name=str(data.get("name", "")),
            description=str(data.get("description", "")),
            category=str(data.get("category", "")),
            driver_count=int(data.get("driver_count", 0)),
            load_count=int(data.get("load_count", 0)),
            default_policy=str(data.get("default_policy", "CFA")),
        )


@dataclass(slots=True)
class ScenarioDetail:
    """Complete scenario configuration including fleet and freight tender lists."""
    summary: ScenarioSummary
    drivers: List[DriverDTO]
    loads: List[LoadDTO]
    cost_config: Optional[CostConfigDTO] = None
    feasibility_config: Optional[FeasibilityConfigDTO] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ScenarioDetail:
        return cls(
            summary=ScenarioSummary.from_dict(data.get("summary", {})),
            drivers=[DriverDTO.from_dict(d) for d in data.get("drivers", [])],
            loads=[LoadDTO.from_dict(l) for l in data.get("loads", [])],
            cost_config=(
                CostConfigDTO.from_dict(data["cost_config"])
                if data.get("cost_config")
                else None
            ),
            feasibility_config=(
                FeasibilityConfigDTO.from_dict(data["feasibility_config"])
                if data.get("feasibility_config")
                else None
            ),
        )


@dataclass(slots=True)
class ValuationBreakdown:
    """Multi-component economic attribution score."""
    total_score: float
    gross_revenue: float
    direct_cost: float
    net_margin: float
    cfa_adjustment: float
    vfa_downstream_value: float
    competitor_risk_premium: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ValuationBreakdown:
        return cls(
            total_score=float(data.get("total_score", 0.0)),
            gross_revenue=float(data.get("gross_revenue", 0.0)),
            direct_cost=float(data.get("direct_cost", 0.0)),
            net_margin=float(data.get("net_margin", 0.0)),
            cfa_adjustment=float(data.get("cfa_adjustment", 0.0)),
            vfa_downstream_value=float(data.get("vfa_downstream_value", 0.0)),
            competitor_risk_premium=float(data.get("competitor_risk_premium", 0.0)),
        )


@dataclass(slots=True)
class ArcExplanation:
    """Detailed attribution analysis for a single driver-load candidate match."""
    driver_id: str
    load_id: str
    status: str
    rank: int
    score_delta_to_winner: float
    rejection_reason: str
    valuation: ValuationBreakdown

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ArcExplanation:
        return cls(
            driver_id=str(data.get("driver_id", "")),
            load_id=str(data.get("load_id", "")),
            status=str(data.get("status", "")),
            rank=int(data.get("rank", 0)),
            score_delta_to_winner=float(data.get("score_delta_to_winner", 0.0)),
            rejection_reason=str(data.get("rejection_reason", "")),
            valuation=ValuationBreakdown.from_dict(data.get("valuation", {})),
        )


@dataclass(slots=True)
class DecisionExplanation:
    """Full decision explainability report with economic attribution waterfall."""
    decision_id: str
    run_id: str
    policy_class: str
    matched_count: int
    total_net_contribution: float
    evaluated_arcs_count: int
    winning_matches: List[ArcExplanation]
    rejected_alternatives: List[ArcExplanation]
    markdown_summary: str

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> DecisionExplanation:
        return cls(
            decision_id=str(data.get("decision_id", "")),
            run_id=str(data.get("run_id", "")),
            policy_class=str(data.get("policy_class", "")),
            matched_count=int(data.get("matched_count", 0)),
            total_net_contribution=float(data.get("total_net_contribution", 0.0)),
            evaluated_arcs_count=int(data.get("evaluated_arcs_count", 0)),
            winning_matches=[
                ArcExplanation.from_dict(a) for a in data.get("winning_matches", [])
            ],
            rejected_alternatives=[
                ArcExplanation.from_dict(a) for a in data.get("rejected_alternatives", [])
            ],
            markdown_summary=str(data.get("markdown_summary", "")),
        )


@dataclass(slots=True)
class ReplayScorecard:
    """Cryptographic deterministic state replay verification result."""
    decision_id: str
    policy_class: str
    status: str
    drift_amount: float
    initial_state_hash_match: bool
    action_hash_match: bool
    recorded_total_net: float
    replayed_total_net: float
    recorded_action_hash: str
    replayed_action_hash: str
    replay_duration_ms: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ReplayScorecard:
        return cls(
            decision_id=str(data.get("decision_id", "")),
            policy_class=str(data.get("policy_class", "")),
            status=str(data.get("status", "")),
            drift_amount=float(data.get("drift_amount", 0.0)),
            initial_state_hash_match=bool(data.get("initial_state_hash_match", False)),
            action_hash_match=bool(data.get("action_hash_match", False)),
            recorded_total_net=float(data.get("recorded_total_net", 0.0)),
            replayed_total_net=float(data.get("replayed_total_net", 0.0)),
            recorded_action_hash=str(data.get("recorded_action_hash", "")),
            replayed_action_hash=str(data.get("replayed_action_hash", "")),
            replay_duration_ms=float(data.get("replay_duration_ms", 0.0)),
        )


@dataclass(slots=True)
class MerkleChainVerification:
    """SHA-256 Merkle chain integrity status."""
    run_id: str
    is_valid: bool
    latest_record_hash: str
    broken_record_id: Optional[str] = None
    verification_message: str = "Chain valid"

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> MerkleChainVerification:
        return cls(
            run_id=str(data.get("run_id", "")),
            is_valid=bool(data.get("is_valid", False)),
            latest_record_hash=str(data.get("latest_record_hash", "")),
            broken_record_id=data.get("broken_record_id"),
            verification_message=str(data.get("verification_message", "Chain valid")),
        )


@dataclass(slots=True)
class RepositionMoveDTO:
    """Synthesized empty tractor relocation."""
    driver_id: str
    origin_node_id: str
    destination_node_id: str
    origin_region_id: str
    destination_region_id: str
    distance_miles: float
    estimated_deadhead_cost: float
    shadow_price_gradient: float
    expected_net_arbitrage: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> RepositionMoveDTO:
        return cls(
            driver_id=str(data.get("driver_id", "")),
            origin_node_id=str(data.get("origin_node_id", "")),
            destination_node_id=str(data.get("destination_node_id", "")),
            origin_region_id=str(data.get("origin_region_id", "")),
            destination_region_id=str(data.get("destination_region_id", "")),
            distance_miles=float(data.get("distance_miles", 0.0)),
            estimated_deadhead_cost=float(data.get("estimated_deadhead_cost", 0.0)),
            shadow_price_gradient=float(data.get("shadow_price_gradient", 0.0)),
            expected_net_arbitrage=float(data.get("expected_net_arbitrage", 0.0)),
        )


@dataclass(slots=True)
class RepositionPlanResponse:
    """Recommended fleet repositioning moves and regional shadow gradients."""
    epoch: int
    moves_count: int
    moves: List[RepositionMoveDTO]
    total_deadhead_cost: float
    total_expected_arbitrage: float
    regional_shadow_prices: Dict[str, float]
    execution_duration_ms: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> RepositionPlanResponse:
        return cls(
            epoch=int(data.get("epoch", 0)),
            moves_count=int(data.get("moves_count", 0)),
            moves=[RepositionMoveDTO.from_dict(m) for m in data.get("moves", [])],
            total_deadhead_cost=float(data.get("total_deadhead_cost", 0.0)),
            total_expected_arbitrage=float(data.get("total_expected_arbitrage", 0.0)),
            regional_shadow_prices={
                k: float(v) for k, v in data.get("regional_shadow_prices", {}).items()
            },
            execution_duration_ms=float(data.get("execution_duration_ms", 0.0)),
        )
