import React, { useState, useEffect } from 'react';
import { Layers, Truck, Compass, CheckCircle2, AlertCircle } from 'lucide-react';
import { apiClient } from '../../api/client';
import { RepositioningMoveDTO } from '../../types/api';

export const RepositioningView: React.FC = () => {
  const [computing, setComputing] = useState(false);
  const [moves, setMoves] = useState<RepositioningMoveDTO[]>([]);
  const [summary, setSummary] = useState<string>('');
  const [error, setError] = useState<string | null>(null);

  const defaultRegionalBalances = [
    { region: 'Midwest (CHI, DET, IND)', surplus: 8, deficit: 0, shadowPrice: -1.25, status: 'SURPLUS' },
    { region: 'Southeast (ATL, CLT, BNA)', surplus: 0, deficit: 6, shadowPrice: +2.40, status: 'DEFICIT' },
    { region: 'Northeast (NYC, BOS, PHL)', surplus: 0, deficit: 4, shadowPrice: +1.85, status: 'DEFICIT' },
    { region: 'South Central (DAL, MEM, STL)', surplus: 3, deficit: 0, shadowPrice: -0.80, status: 'SURPLUS' },
    { region: 'West Coast (LAX, SEA, PHX)', surplus: 2, deficit: 2, shadowPrice: 0.00, status: 'BALANCED' },
  ];

  const fetchRepositionPlan = async () => {
    setComputing(true);
    setError(null);
    try {
      // Build standard sample network
      const baseEpoch = Math.floor(Date.now() / 1000);
      const res = await apiClient.repositionPlan({
        drivers: [
          { id: 'DRV-MW-01', current_location: { node_id: 'CHI', lat: 41.8781, lon: -87.6298 }, home_location: { node_id: 'CHI', lat: 41.8781, lon: -87.6298 }, available_epoch: baseEpoch, drive_hours_remaining: 11.0, duty_hours_remaining: 14.0 },
          { id: 'DRV-MW-02', current_location: { node_id: 'CHI', lat: 41.8781, lon: -87.6298 }, home_location: { node_id: 'CHI', lat: 41.8781, lon: -87.6298 }, available_epoch: baseEpoch, drive_hours_remaining: 11.0, duty_hours_remaining: 14.0 },
          { id: 'DRV-MW-03', current_location: { node_id: 'DET', lat: 42.3314, lon: -83.0458 }, home_location: { node_id: 'DET', lat: 42.3314, lon: -83.0458 }, available_epoch: baseEpoch, drive_hours_remaining: 11.0, duty_hours_remaining: 14.0 },
          { id: 'DRV-MW-04', current_location: { node_id: 'IND', lat: 39.7684, lon: -86.1581 }, home_location: { node_id: 'IND', lat: 39.7684, lon: -86.1581 }, available_epoch: baseEpoch, drive_hours_remaining: 11.0, duty_hours_remaining: 14.0 },
          { id: 'DRV-SC-01', current_location: { node_id: 'DAL', lat: 32.7767, lon: -96.7970 }, home_location: { node_id: 'DAL', lat: 32.7767, lon: -96.7970 }, available_epoch: baseEpoch, drive_hours_remaining: 11.0, duty_hours_remaining: 14.0 },
        ],
        loads: [
          { id: 'LOAD-ATL-CHI', origin: { node_id: 'ATL', lat: 33.7490, lon: -84.3880 }, destination: { node_id: 'CHI', lat: 41.8781, lon: -87.6298 }, pickup_earliest_epoch: baseEpoch + 36000, pickup_latest_epoch: baseEpoch + 72000, delivery_earliest_epoch: baseEpoch + 72000, delivery_latest_epoch: baseEpoch + 108000, revenue: 2800.0, required_equipment: 'DRY_VAN' },
          { id: 'LOAD-CLT-NYC', origin: { node_id: 'CLT', lat: 35.2271, lon: -80.8431 }, destination: { node_id: 'NYC', lat: 40.7128, lon: -74.0060 }, pickup_earliest_epoch: baseEpoch + 36000, pickup_latest_epoch: baseEpoch + 72000, delivery_earliest_epoch: baseEpoch + 72000, delivery_latest_epoch: baseEpoch + 108000, revenue: 2600.0, required_equipment: 'DRY_VAN' },
        ],
      });
      setMoves(res.moves || []);
      setSummary(res.summary || '');
    } catch (err: any) {
      setError(err.message || 'Failed computing repositioning moves');
    } finally {
      setComputing(false);
    }
  };

  useEffect(() => {
    fetchRepositionPlan();
  }, []);

  return (
    <div className="space-y-6">
      {/* Top Banner */}
      <div className="p-6 rounded-3xl bg-gradient-to-r from-slate-900 via-blue-950/20 to-slate-900 border border-slate-800 shadow-xl flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center space-x-2">
            <Layers className="w-5 h-5 text-cyan-400" />
            <h2 className="text-base font-extrabold text-white tracking-tight font-mono uppercase">
              Fleet Network Rebalancing & Telemetry Ingestion
            </h2>
          </div>
          <p className="text-xs text-slate-400 font-mono mt-1 max-w-2xl leading-relaxed">
            Synthesizes empty repositioning moves using dual shadow price gradients to proactively reposition unassigned tractors into high-yield demand nodes.
          </p>
        </div>

        <button
          onClick={fetchRepositionPlan}
          disabled={computing}
          className="flex items-center space-x-2 px-5 py-2.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-mono font-bold transition shadow-lg shadow-cyan-600/20 disabled:opacity-50"
        >
          <Compass className={`w-3.5 h-3.5 ${computing ? 'animate-spin' : ''}`} />
          <span>{computing ? 'Computing Gradients...' : 'Recompute Shadow Gradients'}</span>
        </button>
      </div>

      {error && (
        <div className="p-4 rounded-2xl bg-red-950/40 border border-red-500/40 text-red-300 text-xs font-mono flex items-start space-x-3">
          <AlertCircle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
          <div>
            <p className="font-bold">Repositioning Optimization Error</p>
            <p className="text-slate-400 mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* Regional Imbalance Table */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="p-6 rounded-3xl bg-slate-900/90 border border-slate-800 shadow-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Compass className="w-4 h-4 text-cyan-400" />
              <h3 className="text-xs font-bold text-white uppercase tracking-wider font-mono">
                Regional Driver Imbalances & Shadow Prices
              </h3>
            </div>
            <span className="text-[11px] font-mono text-slate-500">5 Active Super-Regions</span>
          </div>

          <div className="space-y-3 font-mono text-xs">
            {defaultRegionalBalances.map((reg) => (
              <div
                key={reg.region}
                className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800/80 flex items-center justify-between"
              >
                <div>
                  <p className="font-bold text-slate-200">{reg.region}</p>
                  <div className="flex items-center space-x-2 mt-1">
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                        reg.status === 'DEFICIT'
                          ? 'bg-rose-950/60 text-rose-300 border border-rose-500/40'
                          : reg.status === 'SURPLUS'
                          ? 'bg-cyan-950/60 text-cyan-300 border border-cyan-500/40'
                          : 'bg-slate-800 text-slate-400'
                      }`}
                    >
                      {reg.status}
                    </span>
                    <span className="text-slate-400 text-[11px]">
                      {reg.surplus > 0 ? `+${reg.surplus} Unassigned` : `-${reg.deficit} Unmet Loads`}
                    </span>
                  </div>
                </div>
                <div className="text-right">
                  <span className="text-slate-500 text-[10px] uppercase">Shadow Price</span>
                  <p
                    className={`text-sm font-extrabold ${
                      reg.shadowPrice > 0 ? 'text-emerald-400' : reg.shadowPrice < 0 ? 'text-slate-400' : 'text-slate-500'
                    }`}
                  >
                    {reg.shadowPrice > 0 ? '+' : ''}${reg.shadowPrice.toFixed(2)}/mi
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Recommended Empty Repositioning Moves from Go Backend */}
        <div className="p-6 rounded-3xl bg-slate-900/90 border border-slate-800 shadow-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Truck className="w-4 h-4 text-emerald-400" />
              <h3 className="text-xs font-bold text-white uppercase tracking-wider font-mono">
                Optimal Empty Tractor Moves ({moves.length})
              </h3>
            </div>
            <span className="text-[11px] font-mono text-emerald-400 font-bold flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5" /> HOS Cleared
            </span>
          </div>

          {summary && (
            <div className="p-3 rounded-xl bg-slate-950/70 border border-slate-800 text-xs font-mono text-cyan-300">
              {summary}
            </div>
          )}

          <div className="space-y-2.5 font-mono text-xs">
            {moves.length === 0 && !computing && (
              <div className="p-6 rounded-2xl bg-slate-950/50 border border-slate-800/60 text-center text-slate-500">
                Network is currently regionally balanced. No empty repositioning moves required.
              </div>
            )}

            {moves.map((m) => (
              <div
                key={`${m.driver_id}-${m.destination_node}`}
                className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800 hover:border-slate-700 transition"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2">
                    <span className="font-bold text-cyan-300">{m.driver_id}</span>
                    <span className="text-slate-500 font-sans">→</span>
                    <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-bold">
                      {m.origin_node} ({m.origin_region}) → {m.destination_node} ({m.destination_region})
                    </span>
                  </div>
                  <span className="text-emerald-400 font-extrabold">+${m.expected_regional_lift.toFixed(2)} Lift</span>
                </div>
                <div className="flex items-center justify-between mt-2 text-[11px] text-slate-400">
                  <span>Deadhead: {m.empty_miles.toFixed(0)} mi (${m.estimated_cost.toFixed(2)})</span>
                  <span className="text-cyan-300 font-semibold">
                    Transit: {m.transit_hours.toFixed(1)}h
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};
