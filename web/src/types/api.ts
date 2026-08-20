export interface LocationDTO {
  node_id: string;
  lat: number;
  lon: number;
}

export interface EquipmentDTO {
  type: string;
}

export interface DriverDTO {
  id: string;
  current_location: LocationDTO;
  home_location: LocationDTO;
  available_epoch: number;
  drive_hours_remaining?: number;
  duty_hours_remaining?: number;
  equipment?: EquipmentDTO;
}

export interface LoadDTO {
  id: string;
  origin: LocationDTO;
  destination: LocationDTO;
  pickup_earliest_epoch: number;
  pickup_latest_epoch: number;
  delivery_earliest_epoch: number;
  delivery_latest_epoch: number;
  revenue: number;
  required_equipment?: string;
}

export interface FacilityDTO {
  id: string;
  name?: string;
  location: LocationDTO;
  type: string;
  average_dwell_minutes?: number;
}

export interface CostConfigDTO {
  fixed_cost_per_load: number;
  loaded_mile_rate: number;
  empty_mile_rate: number;
  empty_to_home_rate: number;
  early_arrival_per_hour: number;
  late_delivery_per_hour: number;
  driver_bonus_weight?: number;
}

export interface FeasibilityConfigDTO {
  max_deadhead_miles: number;
  max_early_dwell_hours?: number;
  max_late_delivery_hours?: number;
  average_speed_mph?: number;
}

export interface MatchDTO {
  driver_id: string;
  load_id: string;
  dispatch_epoch: number;
  estimated_contribution: number;
}

export interface OptimizeRequest {
  epoch: number;
  drivers: DriverDTO[];
  loads: LoadDTO[];
  policy_class?: string;
  competitor_scale?: number;
  cost_config?: CostConfigDTO;
  feasibility_config?: FeasibilityConfigDTO;
}

export interface OptimizeResponse {
  decision_id: string;
  run_id: string;
  epoch: number;
  match_count: number;
  matches: MatchDTO[];
  total_net_contribution: number;
  execution_duration_ms: number;
}

export interface ScenarioSummaryDTO {
  id: string;
  name: string;
  description: string;
  category: string;
  driver_count: number;
  load_count: number;
  default_policy: string;
}

export interface ScenarioDetailDTO {
  summary: ScenarioSummaryDTO;
  drivers: DriverDTO[];
  loads: LoadDTO[];
  cost_config?: CostConfigDTO;
  feasibility_config?: FeasibilityConfigDTO;
}

export interface EconomicBreakdownDTO {
  gross_revenue: number;
  loaded_drive_cost: number;
  empty_deadhead_cost: number;
  empty_to_home_cost: number;
  inserted_dwell_cost: number;
  late_penalty: number;
  driver_bonus: number;
  immediate_net_margin: number;
  cfa_adjustment?: number;
  downstream_regional_vfa?: number;
  competitor_risk_premium?: number;
  total_objective_score: number;
}

export interface CounterfactualAlternativeDTO {
  load_id: string;
  score_delta: number;
  total_score: number;
  economic_breakdown: EconomicBreakdownDTO;
  rejection_reason: string;
  post_decision_region: string;
  deadhead_miles: number;
  loaded_miles: number;
}

export interface MatchExplanationDTO {
  driver_id: string;
  assigned_load_id: string;
  dispatch_epoch: number;
  winning_score: number;
  immediate_net_margin: number;
  economic_breakdown: EconomicBreakdownDTO;
  summary: string;
  deadhead_miles: number;
  loaded_miles: number;
  inserted_dwell_minutes: number;
  inserted_rest_minutes: number;
  post_decision_region: string;
  rejected_alternatives?: CounterfactualAlternativeDTO[];
}

export interface IdleDriverExplanationDTO {
  driver_id: string;
  reason_code: string;
  summary: string;
  evaluated_candidates_count: number;
  best_candidate_alternative?: CounterfactualAlternativeDTO;
}

export interface DecisionExplanationDTO {
  decision_id: string;
  optimization_run_id: string;
  batch_epoch: number;
  policy_name: string;
  total_drivers: number;
  matched_drivers_count: number;
  idle_drivers_count: number;
  total_net_contribution: number;
  total_objective_value: number;
  executive_summary: string;
  matched_explanations: MatchExplanationDTO[];
  idle_explanations: IdleDriverExplanationDTO[];
}

export interface ExplainResponseDTO {
  decision_id: string;
  explanation: DecisionExplanationDTO;
  markdown: string;
}

export interface ReplayReportDTO {
  decision_id: string;
  run_id: string;
  epoch: number;
  policy_name: string;
  is_bit_exact: boolean;
  initial_state_hash_match: boolean;
  action_hash_match: boolean;
  recorded_action_hash: string;
  replayed_action_hash: string;
  recorded_matches_count: number;
  replayed_matches_count: number;
  recorded_net_contribution: number;
  replayed_net_contribution: number;
  contribution_delta: number;
  replay_duration_us: number;
  drift_details: string[];
}

export interface ChainIntegrityResponseDTO {
  run_id: string;
  is_valid: boolean;
  latest_record_hash: string;
  broken_record_id: string;
  status: string;
}

export interface DailyKPISnapshotDTO {
  day_index: number;
  epoch: number;
  active_drivers: number;
  total_loaded_miles: number;
  total_empty_miles: number;
  empty_mile_ratio: number;
  gross_revenue: number;
  total_cost: number;
  net_contribution: number;
  direct_tour_count: number;
  relay_exchange_count: number;
}

export interface SimulateRequestDTO {
  run_id: string;
  start_epoch: number;
  horizon_days: number;
  decision_step_hours?: number;
  enable_relays?: boolean;
  min_relay_haul_miles?: number;
  drivers: DriverDTO[];
  facilities?: FacilityDTO[];
  load_schedule: LoadDTO[];
}

export interface SimulateResponseDTO {
  run_id: string;
  total_days: number;
  total_epochs: number;
  cumulative_loaded_miles: number;
  cumulative_empty_miles: number;
  overall_empty_ratio: number;
  cumulative_gross_revenue: number;
  cumulative_cost: number;
  cumulative_net_contribution: number;
  daily_kpis: DailyKPISnapshotDTO[];
}

export interface RepositioningMoveDTO {
  driver_id: string;
  origin_location: LocationDTO;
  target_region_id: string;
  target_location: LocationDTO;
  start_epoch: number;
  arrival_epoch: number;
  deadhead_miles: number;
  estimated_cost: number;
  expected_arbitrage_yield: number;
  net_repositioning_value: number;
}

export interface RepositionPlanRequestDTO {
  drivers: DriverDTO[];
  loads: LoadDTO[];
  config?: any;
}

export interface RepositionPlanResponseDTO {
  moves: RepositioningMoveDTO[];
  total_moves: number;
  summary: string;
}
