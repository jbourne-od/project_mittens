import React, { useEffect, useState } from 'react';
import { Zap, ShieldCheck, Server, RefreshCw } from 'lucide-react';
import { apiClient } from '../api/client';

export const Header: React.FC = () => {
  const [health, setHealth] = useState<{ status: string; version: string; latencyMs: number } | null>(null);
  const [loading, setLoading] = useState(false);

  const checkStatus = async () => {
    setLoading(true);
    try {
      const res = await apiClient.checkHealth();
      setHealth(res);
    } catch {
      setHealth(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    checkStatus();
    const interval = setInterval(checkStatus, 15000);
    return () => clearInterval(interval);
  }, []);

  return (
    <header className="bg-slate-900/90 border-b border-slate-800 backdrop-blur-md sticky top-0 z-40 px-6 py-3.5 flex items-center justify-between shadow-lg">
      <div className="flex items-center space-x-3">
        <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center shadow-md shadow-cyan-500/20 text-white font-black text-xl">
          M
        </div>
        <div>
          <div className="flex items-center space-x-2">
            <h1 className="font-extrabold text-lg text-white tracking-tight flex items-center gap-2">
              PROJECT MITTENS
              <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/30">
                MISSION CONTROL
              </span>
            </h1>
          </div>
          <p className="text-xs text-slate-400 font-mono">
            High-Efficiency POMDP Carrier Optimization Engine & Provenance Spine
          </p>
        </div>
      </div>

      <div className="flex items-center space-x-4">
        {/* Engine Specs Pin */}
        <div className="hidden md:flex items-center space-x-2 px-3 py-1.5 rounded-lg bg-slate-800/80 border border-slate-700/60 text-xs font-mono">
          <Zap className="w-3.5 h-3.5 text-amber-400" />
          <span className="text-slate-400">Runtime:</span>
          <span className="text-slate-200 font-semibold">mittens-v1.2.0</span>
          <span className="text-slate-500">•</span>
          <span className="text-emerald-400 font-semibold">LAPJV $O(N^3)$</span>
        </div>

        {/* Backend API Health Status */}
        <div className="flex items-center space-x-2 px-3 py-1.5 rounded-lg bg-slate-800/80 border border-slate-700/60 text-xs font-mono">
          <Server className="w-3.5 h-3.5 text-cyan-400" />
          <span className="text-slate-400">Go REST API:</span>
          {health ? (
            <div className="flex items-center space-x-1.5">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
              <span className="text-emerald-400 font-semibold">ONLINE ({health.latencyMs}ms)</span>
            </div>
          ) : (
            <div className="flex items-center space-x-1.5">
              <span className="w-2 h-2 rounded-full bg-red-400"></span>
              <span className="text-red-400 font-semibold">OFFLINE</span>
            </div>
          )}
          <button
            onClick={checkStatus}
            title="Refresh backend health"
            className="text-slate-400 hover:text-cyan-400 transition ml-1"
          >
            <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          </button>
        </div>

        {/* Cryptographic Proof Badge */}
        <div className="hidden lg:flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-emerald-950/40 border border-emerald-500/30 text-emerald-400 text-xs font-mono">
          <ShieldCheck className="w-4 h-4 text-emerald-400" />
          <span className="font-semibold">MERKLE SPINE RATIFIED</span>
        </div>
      </div>
    </header>
  );
};
