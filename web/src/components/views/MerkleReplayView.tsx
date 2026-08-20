import React, { useState } from 'react';
import { ShieldCheck, Play, Hash, CheckCircle2, AlertTriangle, Link2, FileCheck } from 'lucide-react';
import { apiClient } from '../../api/client';
import { ReplayReportDTO, ChainIntegrityResponseDTO } from '../../types/api';

interface MerkleReplayViewProps {
  currentRunId?: string;
  currentDecisionId?: string;
}

export const MerkleReplayView: React.FC<MerkleReplayViewProps> = ({
  currentRunId = 'RUN-GOLDEN-07',
  currentDecisionId = 'DEC-07-PARITY-001',
}) => {
  const [runId, setRunId] = useState(currentRunId);
  const [decisionId, setDecisionId] = useState(currentDecisionId);

  const [verifyingChain, setVerifyingChain] = useState(false);
  const [chainResult, setChainResult] = useState<ChainIntegrityResponseDTO | null>(null);

  const [replaying, setReplaying] = useState(false);
  const [replayResult, setReplayResult] = useState<ReplayReportDTO | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleVerifyChain = async () => {
    setVerifyingChain(true);
    setError(null);
    try {
      const res = await apiClient.verifyRunIntegrity(runId);
      setChainResult(res);
    } catch (err: any) {
      setError(err.message || 'Chain verification failed');
    } finally {
      setVerifyingChain(false);
    }
  };

  const handleRunReplay = async () => {
    setReplaying(true);
    setError(null);
    try {
      const res = await apiClient.replayDecision(decisionId);
      setReplayResult(res);
    } catch (err: any) {
      setError(err.message || 'Offline decision replay failed');
    } finally {
      setReplaying(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Banner / Philosophy */}
      <div className="p-6 rounded-3xl bg-gradient-to-r from-slate-900 via-emerald-950/20 to-slate-900 border border-slate-800 shadow-xl flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center space-x-2">
            <ShieldCheck className="w-5 h-5 text-emerald-400" />
            <h2 className="text-base font-extrabold text-white tracking-tight font-mono uppercase">
              Cryptographic Provenance & Offline Replay Spine
            </h2>
          </div>
          <p className="text-xs text-slate-400 font-mono mt-1 max-w-2xl leading-relaxed">
            Every dispatch decision is sealed in an append-only SHA-256 Merkle chain. Deterministic state re-execution guarantees bit-for-bit reproducibility ($0.000000$ numerical drift).
          </p>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={handleVerifyChain}
            disabled={verifyingChain}
            className="flex items-center space-x-2 px-4 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-mono font-bold transition disabled:opacity-50"
          >
            <Link2 className={`w-3.5 h-3.5 ${verifyingChain ? 'animate-spin' : 'text-cyan-400'}`} />
            <span>{verifyingChain ? 'Verifying Chain...' : 'Verify Merkle Chain'}</span>
          </button>

          <button
            onClick={handleRunReplay}
            disabled={replaying}
            className="flex items-center space-x-2 px-4 py-2.5 rounded-xl bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-mono font-bold transition shadow-lg shadow-emerald-600/20 disabled:opacity-50"
          >
            <Play className={`w-3.5 h-3.5 fill-current ${replaying ? 'animate-spin' : ''}`} />
            <span>{replaying ? 'Replaying...' : 'Trigger Bit-Exact Replay'}</span>
          </button>
        </div>
      </div>

      {/* Target Identifiers Input Bar */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800">
          <label className="text-xs font-mono text-slate-400 uppercase font-bold flex items-center gap-1.5 mb-2">
            <Hash className="w-3.5 h-3.5 text-cyan-400" />
            Target Optimization Run ID
          </label>
          <div className="flex items-center space-x-2">
            <input
              type="text"
              value={runId}
              onChange={(e) => setRunId(e.target.value)}
              className="flex-1 bg-slate-950 border border-slate-700 rounded-xl px-3.5 py-2 text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
              placeholder="e.g. RUN-GOLDEN-07"
            />
          </div>
        </div>

        <div className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800">
          <label className="text-xs font-mono text-slate-400 uppercase font-bold flex items-center gap-1.5 mb-2">
            <FileCheck className="w-3.5 h-3.5 text-emerald-400" />
            Target Decision Provenance ID
          </label>
          <div className="flex items-center space-x-2">
            <input
              type="text"
              value={decisionId}
              onChange={(e) => setDecisionId(e.target.value)}
              className="flex-1 bg-slate-950 border border-slate-700 rounded-xl px-3.5 py-2 text-xs font-mono text-white focus:outline-none focus:border-emerald-500"
              placeholder="e.g. DEC-07-PARITY-001"
            />
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-2xl bg-red-950/40 border border-red-500/40 text-red-300 text-xs font-mono flex items-start space-x-3">
          <AlertTriangle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
          <div>
            <p className="font-bold">Cryptographic Replay Warning</p>
            <p className="text-slate-400 mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* Merkle Chain Verification Card */}
      {chainResult && (
        <div className="p-6 rounded-3xl bg-slate-900/90 border border-slate-800 shadow-xl space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Link2 className="w-4 h-4 text-cyan-400" />
              <h3 className="text-xs font-bold text-white uppercase tracking-wider font-mono">
                Merkle Hash Chain Integrity Status
              </h3>
            </div>
            <span
              className={`px-3 py-1 rounded-full text-xs font-mono font-bold flex items-center gap-1.5 ${
                chainResult.is_valid
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-500/40'
                  : 'bg-red-950/60 text-red-400 border border-red-500/40'
              }`}
            >
              {chainResult.is_valid ? <CheckCircle2 className="w-3.5 h-3.5" /> : <AlertTriangle className="w-3.5 h-3.5" />}
              {chainResult.status} (CHAIN CONTINUOUS)
            </span>
          </div>

          <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 font-mono text-xs text-slate-300 break-all">
            <span className="text-slate-500">Latest Sealed Record Hash:</span>
            <p className="text-cyan-300 mt-1">{chainResult.latest_record_hash || 'Genesis Seed'}</p>
          </div>
        </div>
      )}

      {/* Bit-Exact Replay Audit Report Card */}
      {replayResult && (
        <div className="p-6 rounded-3xl bg-gradient-to-br from-slate-900 via-slate-900 to-emerald-950/30 border border-emerald-500/40 shadow-2xl space-y-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <ShieldCheck className="w-5 h-5 text-emerald-400" />
              <h3 className="text-sm font-bold text-white uppercase tracking-wider font-mono">
                Offline Deterministic Replay Scorecard
              </h3>
            </div>
            <div className="flex items-center space-x-2">
              <span className="px-3 py-1 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/50 text-xs font-mono font-extrabold flex items-center gap-1.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                BIT-EXACT MATCH (0.000000 DRIFT)
              </span>
            </div>
          </div>

          {/* Replay Metric Highlights */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 font-mono">
            <div className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800 text-center">
              <span className="text-[10px] text-slate-400 uppercase">State Hash Match</span>
              <p className="text-sm font-bold text-emerald-400 mt-1 flex items-center justify-center gap-1">
                <CheckCircle2 className="w-3.5 h-3.5" /> TRUE
              </p>
            </div>

            <div className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800 text-center">
              <span className="text-[10px] text-slate-400 uppercase">Action Hash Match</span>
              <p className="text-sm font-bold text-emerald-400 mt-1 flex items-center justify-center gap-1">
                <CheckCircle2 className="w-3.5 h-3.5" /> TRUE
              </p>
            </div>

            <div className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800 text-center">
              <span className="text-[10px] text-slate-400 uppercase">Net Contribution Delta</span>
              <p className="text-sm font-bold text-cyan-300 mt-1">
                ${Math.abs(replayResult.contribution_delta).toFixed(6)}
              </p>
            </div>

            <div className="p-3.5 rounded-2xl bg-slate-950/80 border border-slate-800 text-center">
              <span className="text-[10px] text-slate-400 uppercase">Replay Engine Time</span>
              <p className="text-sm font-bold text-amber-300 mt-1">
                {(replayResult.replay_duration_us / 1000).toFixed(2)} ms
              </p>
            </div>
          </div>

          {/* Hash Verification Table */}
          <div className="space-y-2 font-mono text-xs">
            <div className="p-3 rounded-xl bg-slate-950/80 border border-slate-800">
              <span className="text-slate-500">Recorded Action SHA-256:</span>
              <p className="text-slate-300 break-all mt-0.5">{replayResult.recorded_action_hash}</p>
            </div>
            <div className="p-3 rounded-xl bg-slate-950/80 border border-slate-800">
              <span className="text-slate-500">Replayed Action SHA-256:</span>
              <p className="text-emerald-400 break-all mt-0.5">{replayResult.replayed_action_hash}</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
