import {
  OptimizeRequest,
  OptimizeResponse,
  ScenarioSummaryDTO,
  ScenarioDetailDTO,
  ExplainResponseDTO,
  ReplayReportDTO,
  ChainIntegrityResponseDTO,
  SimulateRequestDTO,
  SimulateResponseDTO,
  RepositionPlanRequestDTO,
  RepositionPlanResponseDTO,
} from '../types/api';

const API_BASE = '/api/v1';

class ApiClient {
  private async request<T>(path: string, options?: RequestInit): Promise<{ data: T; latencyMs: number }> {
    const start = performance.now();
    try {
      const res = await fetch(`${API_BASE}${path}`, {
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
        ...options,
      });

      const latencyMs = Math.round((performance.now() - start) * 100) / 100;

      if (!res.ok) {
        let errorMsg = `HTTP Error ${res.status}: ${res.statusText}`;
        try {
          const errBody = await res.json();
          if (errBody.message) errorMsg = errBody.message;
        } catch {
          // ignore json parse error
        }
        throw new Error(errorMsg);
      }

      const data = (await res.json()) as T;
      return { data, latencyMs };
    } catch (err: any) {
      throw new Error(err.message || 'Network request failed');
    }
  }

  async checkHealth(): Promise<{ status: string; version: string; uptime_seconds: number; latencyMs: number }> {
    const start = performance.now();
    const res = await fetch('/healthz');
    const latencyMs = Math.round((performance.now() - start) * 100) / 100;
    if (!res.ok) throw new Error('Health check failed');
    const data = await res.json();
    return { ...data, latencyMs };
  }

  async listScenarios(): Promise<{ scenarios: ScenarioSummaryDTO[]; count: number }> {
    const { data } = await this.request<{ scenarios: ScenarioSummaryDTO[]; count: number }>('/scenarios');
    return data;
  }

  async getScenario(id: string): Promise<ScenarioDetailDTO> {
    const { data } = await this.request<ScenarioDetailDTO>(`/scenarios/${id}`);
    return data;
  }

  async optimize(payload: OptimizeRequest): Promise<{ response: OptimizeResponse; latencyMs: number }> {
    const { data, latencyMs } = await this.request<OptimizeResponse>('/optimize', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    return { response: data, latencyMs };
  }

  async getExplanation(decisionId: string): Promise<ExplainResponseDTO> {
    const { data } = await this.request<ExplainResponseDTO>(`/decisions/${decisionId}/explain`);
    return data;
  }

  async replayDecision(decisionId: string): Promise<ReplayReportDTO> {
    const { data } = await this.request<ReplayReportDTO>(`/decisions/${decisionId}/replay`, {
      method: 'POST',
    });
    return data;
  }

  async verifyRunIntegrity(runId: string): Promise<ChainIntegrityResponseDTO> {
    const { data } = await this.request<ChainIntegrityResponseDTO>(`/runs/${runId}/integrity`);
    return data;
  }

  async simulate(payload: SimulateRequestDTO): Promise<SimulateResponseDTO> {
    const { data } = await this.request<SimulateResponseDTO>('/simulate', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    return data;
  }

  async repositionPlan(payload: RepositionPlanRequestDTO): Promise<RepositionPlanResponseDTO> {
    const { data } = await this.request<RepositionPlanResponseDTO>('/reposition/plan', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    return data;
  }
}

export const apiClient = new ApiClient();
