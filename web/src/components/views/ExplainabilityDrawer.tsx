import React, { useEffect, useState } from 'react';
import { ExplainResponseDTO } from '../../types/api';
import { apiClient } from '../../api/client';
import { X, Sparkles, TrendingUp, DollarSign, ArrowRight, CheckCircle2, AlertCircle, Truck, FileText } from 'lucide-react';

interface ExplainabilityDrawerProps {
  decisionId: string | null;
  selectedDriverId?: string;
  onClose: () => void;
}

export const ExplainabilityDrawer: React.FC<ExplainabilityDrawerProps> = ({
  decisionId,
  selectedDriverId,
  onClose,
}) => {
  const [data, setData] = useState<ExplainResponseDTO | null>(null);
  const [activeDriverIndex, setActiveDriverIndex] = useState<number>(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showRawMarkdown, setShowRawMarkdown] = useState(false);

  useEffect(() => {
    if (!decisionId) return;
    setLoading(true);
    setError(null);
    apiClient
      .getExplanation(decisionId)
      .then((res) => {
        setData(res);
        if (selectedDriverId && res.explanation?.matched_explanations) {
          const idx = res.explanation.matched_explanations.findIndex((m) => m.driver_id === selectedDriverId);
          if (idx >= 0) setActiveDriverIndex(idx);
        } else {
          setActiveDriverIndex(0);
        }
      })
      .catch((err) => {
        setError(err.message || 'Failed to load explainability breakdown');
      })
      .finally(() => {
        setLoading(false);
      });
  }, [decisionId, selectedDriverId]);

  if (!decisionId) return null;

  const currentMatch = data?.explanation?.matched_explanations?.[activeDriverIndex] || data?.explanation?.matched_explanations?.[0];
  const breakdown = currentMatch?.economic_breakdown;

  return (
    <div className="fixed inset-y-0 right-0 z-50 w-full max-w-2xl bg-slate-900/95 border-l border-slate-800 shadow-2xl backdrop-blur-xl flex flex-col transform transition-transform duration-300 ease-in-out">
      {/* Drawer Header */}
      <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/60">
        <div className="flex items-center space-x-2.5">
          <div className="w-8 h-8 rounded-lg bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
            <Sparkles className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-white font-mono uppercase tracking-tight flex items-center gap-2">
              Decision Explainability & Attribution
            </h2>
            <p className="text-xs text-slate-400 font-mono">
              Decision ID: <span className="text-cyan-300 font-semibold">{decisionId}</span>
            </p>
          </div>
        </div>
        <div className="flex items-center space-x-2">
          {data?.markdown && (
            <button
              onClick={() => setShowRawMarkdown(!showRawMarkdown)}
              className={`p-1.5 rounded-lg border text-xs font-mono transition flex items-center gap-1 ${
                showRawMarkdown
                  ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              }`}
              title="Toggle Raw Markdown Report"
            >
              <FileText className="w-4 h-4" />
            </button>
          )}
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>
      </div>

      {/* Drawer Body */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {loading && (
          <div className="flex flex-col items-center justify-center h-64 space-y-3">
            <div className="w-8 h-8 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin"></div>
            <p className="text-xs text-slate-400 font-mono">Extracting economic attribution decomposition...</p>
          </div>
        )}

        {error && (
          <div className="p-4 rounded-xl bg-red-950/40 border border-red-500/40 text-red-300 text-xs font-mono flex items-start space-x-3">
            <AlertCircle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
            <div>
              <p className="font-bold">Explainability Not Found</p>
              <p className="text-slate-400 mt-1">{error}</p>
            </div>
          </div>
        )}

        {data && !loading && (
          <>
            {showRawMarkdown ? (
              <div className="p-4 rounded-2xl bg-slate-950/90 border border-slate-800 font-mono text-xs text-slate-300 whitespace-pre-wrap leading-relaxed">
                {data.markdown}
              </div>
            ) : (
              <>
                {/* Driver Match Tabs (if multiple drivers matched in decision) */}
                {data.explanation?.matched_explanations && data.explanation.matched_explanations.length > 1 && (
                  <div className="flex items-center space-x-2 overflow-x-auto pb-1">
                    {data.explanation.matched_explanations.map((m, idx) => (
                      <button
                        key={`driver-tab-${m.driver_id}`}
                        onClick={() => setActiveDriverIndex(idx)}
                        className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-xl text-xs font-mono font-bold transition whitespace-nowrap ${
                          activeDriverIndex === idx
                            ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm'
                            : 'bg-slate-950 text-slate-400 border border-slate-800 hover:text-slate-200'
                        }`}
                      >
                        <Truck className="w-3.5 h-3.5" />
                        <span>{m.driver_id}</span>
                        <span className="text-slate-500">→</span>
                        <span className="text-emerald-400">{m.assigned_load_id}</span>
                      </button>
                    ))}
                  </div>
                )}

                {/* Attribution Summary Banner */}
                {currentMatch && (
                  <div className="p-4 rounded-2xl bg-gradient-to-br from-slate-950 to-slate-900 border border-slate-800 shadow-lg">
                    <div className="grid grid-cols-3 gap-4 text-center font-mono">
                      <div className="p-2.5 rounded-xl bg-slate-900/80 border border-slate-800">
                        <span className="text-[10px] text-slate-400 uppercase">Assigned Driver</span>
                        <p className="text-sm font-bold text-cyan-300 mt-0.5">{currentMatch.driver_id}</p>
                      </div>
                      <div className="p-2.5 rounded-xl bg-slate-900/80 border border-slate-800">
                        <span className="text-[10px] text-slate-400 uppercase">Matched Freight</span>
                        <p className="text-sm font-bold text-emerald-300 mt-0.5">{currentMatch.assigned_load_id}</p>
                      </div>
                      <div className="p-2.5 rounded-xl bg-slate-900/80 border border-slate-800">
                        <span className="text-[10px] text-slate-400 uppercase">Policy Class</span>
                        <p className="text-sm font-bold text-amber-300 mt-0.5">{data.explanation?.policy_name || 'CFA'}</p>
                      </div>
                    </div>
                  </div>
                )}

                {/* Economic Waterfall Decomposition */}
                {breakdown && (
                  <div>
                    <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider font-mono mb-3 flex items-center gap-2">
                      <DollarSign className="w-3.5 h-3.5 text-cyan-400" />
                      Economic Attribution Waterfall
                    </h3>

                    <div className="space-y-2 font-mono text-xs">
                      {/* Revenue */}
                      <div className="flex items-center justify-between p-3 rounded-xl bg-emerald-950/20 border border-emerald-500/20">
                        <span className="text-slate-300">Gross Load Revenue</span>
                        <span className="font-bold text-emerald-400">+${breakdown.gross_revenue.toFixed(2)}</span>
                      </div>

                      {/* Deadhead Cost */}
                      <div className="flex items-center justify-between p-3 rounded-xl bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-400">Empty Deadhead Fuel & Labor ({currentMatch.deadhead_miles || 0} mi)</span>
                        <span className="font-semibold text-rose-400">-${breakdown.empty_deadhead_cost.toFixed(2)}</span>
                      </div>

                      {/* Loaded Haul Cost */}
                      <div className="flex items-center justify-between p-3 rounded-xl bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-400">Loaded Linehaul Direct Cost ({currentMatch.loaded_miles || 0} mi)</span>
                        <span className="font-semibold text-rose-400">-${breakdown.loaded_drive_cost.toFixed(2)}</span>
                      </div>

                      {/* Dwell / Late */}
                      {(breakdown.inserted_dwell_cost > 0 || breakdown.late_penalty > 0) && (
                        <div className="flex items-center justify-between p-3 rounded-xl bg-slate-900/60 border border-slate-800">
                          <span className="text-slate-400">Dwell & Appointment Delay Penalty</span>
                          <span className="font-semibold text-amber-400">
                            -${(breakdown.inserted_dwell_cost + breakdown.late_penalty).toFixed(2)}
                          </span>
                        </div>
                      )}

                      {/* CFA Adjustment */}
                      {breakdown.cfa_adjustment !== undefined && breakdown.cfa_adjustment !== 0 && (
                        <div className="flex items-center justify-between p-3 rounded-xl bg-slate-900/60 border border-slate-800">
                          <span className="text-slate-400">CFA Parametric Adjustment ($\theta_1$)</span>
                          <span className={`font-semibold ${breakdown.cfa_adjustment >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                            {breakdown.cfa_adjustment >= 0 ? '+' : ''}${breakdown.cfa_adjustment.toFixed(2)}
                          </span>
                        </div>
                      )}

                      {/* VFA Value */}
                      {breakdown.downstream_regional_vfa !== undefined && breakdown.downstream_regional_vfa > 0 && (
                        <div className="flex items-center justify-between p-3 rounded-xl bg-cyan-950/20 border border-cyan-500/20">
                          <span className="text-cyan-300">Downstream VFA Destination Value ({currentMatch.post_decision_region})</span>
                          <span className="font-bold text-cyan-400">+${breakdown.downstream_regional_vfa.toFixed(2)}</span>
                        </div>
                      )}

                      {/* Risk Premium */}
                      {breakdown.competitor_risk_premium !== undefined && breakdown.competitor_risk_premium > 0 && (
                        <div className="flex items-center justify-between p-3 rounded-xl bg-amber-950/20 border border-amber-500/20">
                          <span className="text-amber-300">Competitor Spot Win Risk Premium (N=1)</span>
                          <span className="font-bold text-amber-400">-${breakdown.competitor_risk_premium.toFixed(2)}</span>
                        </div>
                      )}

                      {/* Total Score */}
                      <div className="flex items-center justify-between p-4 rounded-xl bg-gradient-to-r from-cyan-950/50 to-blue-950/50 border border-cyan-500/40 shadow-md">
                        <span className="font-bold text-white text-sm">Total Evaluated Contribution</span>
                        <span className="font-extrabold text-cyan-300 text-base">
                          ${breakdown.total_objective_score.toFixed(2)}
                        </span>
                      </div>
                    </div>
                  </div>
                )}

                {/* Plain English Rationalization */}
                {currentMatch?.summary && (
                  <div className="p-4 rounded-2xl bg-slate-950/70 border border-slate-800 text-xs font-mono text-slate-300 leading-relaxed">
                    <p className="text-[11px] uppercase tracking-wider text-slate-400 font-bold mb-1.5 flex items-center gap-1.5">
                      <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                      Mathematical Justification Summary
                    </p>
                    <p className="whitespace-pre-line">{currentMatch.summary}</p>
                  </div>
                )}

                {/* Ranked Counterfactual Alternatives */}
                {currentMatch?.rejected_alternatives && currentMatch.rejected_alternatives.length > 0 && (
                  <div>
                    <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider font-mono mb-3 flex items-center gap-2">
                      <TrendingUp className="w-3.5 h-3.5 text-amber-400" />
                      Rejected Counterfactual Candidate Loads ({currentMatch.rejected_alternatives.length})
                    </h3>

                    <div className="space-y-2 font-mono text-xs">
                      {currentMatch.rejected_alternatives.map((alt, idx) => (
                        <div
                          key={`alt-${alt.load_id}-${idx}`}
                          className="p-3.5 rounded-xl bg-slate-950/80 border border-slate-800 hover:border-slate-700 transition"
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center space-x-2">
                              <span className="px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 font-bold">
                                #{idx + 1}
                              </span>
                              <span className="font-bold text-white">{alt.load_id}</span>
                              <span className="text-slate-500">(Region: {alt.post_decision_region})</span>
                            </div>
                            <span className="text-rose-400 font-semibold">
                              Δ -${Math.abs(alt.score_delta).toFixed(2)}
                            </span>
                          </div>
                          <p className="text-[11px] text-slate-400 mt-2 flex items-start gap-1.5">
                            <ArrowRight className="w-3 h-3 text-slate-600 flex-shrink-0 mt-0.5" />
                            <span>{alt.rejection_reason}</span>
                          </p>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
};
