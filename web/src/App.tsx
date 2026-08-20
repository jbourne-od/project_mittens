import React, { useState } from 'react';
import { Header } from './components/Header';
import { Navigation, TabType } from './components/Navigation';
import { GoldenPlaygroundView } from './components/views/GoldenPlaygroundView';
import { MerkleReplayView } from './components/views/MerkleReplayView';
import { TournamentSimView } from './components/views/TournamentSimView';
import { RepositioningView } from './components/views/RepositioningView';
import { ExplainabilityDrawer } from './components/views/ExplainabilityDrawer';
import { ShieldCheck, Cpu, Code2, Sparkles } from 'lucide-react';

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<TabType>('playground');
  const [lastDecisionId, setLastDecisionId] = useState<string>('');
  const [lastRunId, setLastRunId] = useState<string>('');
  const [searchDecisionId, setSearchDecisionId] = useState<string>('');
  const [showSearchExplain, setShowSearchExplain] = useState<boolean>(false);

  const handleDecisionCreated = (decisionId: string, runId: string) => {
    setLastDecisionId(decisionId);
    setLastRunId(runId);
    setSearchDecisionId(decisionId);
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col selection:bg-cyan-500/30 selection:text-cyan-200">
      {/* Top Header */}
      <Header />

      {/* Tab Navigation */}
      <Navigation
        activeTab={activeTab}
        onSelectTab={setActiveTab}
      />

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-6 space-y-6">
        {activeTab === 'playground' && (
          <GoldenPlaygroundView onDecisionCreated={handleDecisionCreated} />
        )}

        {activeTab === 'explainability' && (
          <div className="space-y-6">
            <div className="p-6 rounded-3xl bg-slate-900/90 border border-slate-800 shadow-xl space-y-4">
              <div className="flex items-center space-x-2">
                <Sparkles className="w-5 h-5 text-cyan-400" />
                <h2 className="text-base font-extrabold text-white tracking-tight font-mono uppercase">
                  Search & Audit Any Dispatch Decision
                </h2>
              </div>
              <p className="text-xs text-slate-400 font-mono">
                Enter any historical Optimization Decision ID to inspect its complete economic waterfall decomposition and counterfactual alternatives.
              </p>
              <div className="flex items-center space-x-3 pt-2">
                <input
                  type="text"
                  value={searchDecisionId || lastDecisionId}
                  onChange={(e) => setSearchDecisionId(e.target.value)}
                  placeholder="e.g. Run Go Optimizer in Playground first"
                  className="flex-1 bg-slate-950 border border-slate-700 rounded-xl px-4 py-2.5 text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
                />
                <button
                  onClick={() => setShowSearchExplain(true)}
                  disabled={!(searchDecisionId || lastDecisionId)}
                  className="px-5 py-2.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-mono font-bold transition shadow-lg shadow-cyan-600/20 disabled:opacity-50"
                >
                  Inspect Waterfall
                </button>
              </div>
            </div>

            {!(searchDecisionId || lastDecisionId) && (
              <div className="p-8 rounded-3xl bg-slate-900/40 border border-slate-800/60 text-center font-mono text-xs text-slate-400">
                <Sparkles className="w-8 h-8 text-cyan-400/60 mx-auto mb-3" />
                <p className="font-bold text-slate-200">No Optimization Run Selected</p>
                <p className="text-slate-500 mt-1">Run an optimization in the Golden Playground or enter a Decision ID above.</p>
              </div>
            )}

            {/* If user triggered explain drawer */}
            <ExplainabilityDrawer
              decisionId={showSearchExplain ? (searchDecisionId || lastDecisionId) : (lastDecisionId || null)}
              onClose={() => setShowSearchExplain(false)}
            />
          </div>
        )}

        {activeTab === 'merkle' && (
          <MerkleReplayView
            currentDecisionId={lastDecisionId}
            currentRunId={lastRunId}
          />
        )}

        {activeTab === 'tournament' && <TournamentSimView />}

        {activeTab === 'reposition' && <RepositioningView />}
      </main>

      {/* Footer / Provenance Seals */}
      <footer className="border-t border-slate-800/80 bg-slate-950/80 py-6 px-8 text-xs font-mono text-slate-400 mt-auto">
        <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex items-center space-x-3">
            <div className="flex items-center space-x-1.5 text-slate-300">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              <span className="font-bold text-white">Project Mittens</span>
              <span className="text-slate-500">|</span>
              <span className="text-slate-400">Ratified MOMDP Carrier Optimization</span>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-4 text-slate-500 text-[11px]">
            <span className="flex items-center gap-1 text-slate-400">
              <Cpu className="w-3.5 h-3.5 text-cyan-400" />
              Lock-Free GMP Go Core
            </span>
            <span>•</span>
            <span className="flex items-center gap-1 text-slate-400">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              Inviolate 1-8 Guaranteed
            </span>
            <span>•</span>
            <span className="flex items-center gap-1 text-slate-400">
              <Code2 className="w-3.5 h-3.5 text-amber-400" />
              Clean Architecture
            </span>
          </div>
        </div>
      </footer>
    </div>
  );
};
