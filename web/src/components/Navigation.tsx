import React from 'react';
import { Play, Sparkles, ShieldCheck, Trophy, Layers } from 'lucide-react';

export type TabType = 'playground' | 'explainability' | 'merkle' | 'tournament' | 'reposition';

interface NavigationProps {
  activeTab: TabType;
  onSelectTab: (tab: TabType) => void;
}

export const Navigation: React.FC<NavigationProps> = ({ activeTab, onSelectTab }) => {
  const tabs = [
    {
      id: 'playground' as TabType,
      label: 'Golden Playground & Map',
      icon: Play,
      badge: '9 Suites Available',
    },
    {
      id: 'explainability' as TabType,
      label: 'Economic Attribution & Waterfall',
      icon: Sparkles,
      badge: 'Why Driver X?',
    },
    {
      id: 'merkle' as TabType,
      label: 'Cryptographic Replay Auditor',
      icon: ShieldCheck,
      badge: '0.000000 Drift',
    },
    {
      id: 'tournament' as TabType,
      label: 'Competitive POMDP Tournament',
      icon: Trophy,
      badge: 'N=0 vs N=1 Lift',
    },
    {
      id: 'reposition' as TabType,
      label: 'Fleet Reposition & Telemetry',
      icon: Layers,
      badge: 'Live Gradients',
    },
  ];

  return (
    <nav className="bg-slate-900 border-b border-slate-800 px-6 py-2 flex items-center space-x-2 overflow-x-auto">
      {tabs.map((tab) => {
        const Icon = tab.icon;
        const isActive = activeTab === tab.id;
        return (
          <button
            key={tab.id}
            onClick={() => onSelectTab(tab.id)}
            className={`flex items-center space-x-2 px-4 py-2 rounded-lg text-xs font-semibold transition-all whitespace-nowrap ${
              isActive
                ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/40 shadow-sm shadow-cyan-500/10'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 border border-transparent'
            }`}
          >
            <Icon className={`w-4 h-4 ${isActive ? 'text-cyan-400' : 'text-slate-400'}`} />
            <span>{tab.label}</span>
            {tab.badge && (
              <span
                className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${
                  isActive
                    ? 'bg-cyan-500/20 text-cyan-200 border border-cyan-500/30'
                    : 'bg-slate-800 text-slate-400 border border-slate-700/50'
                }`}
              >
                {tab.badge}
              </span>
            )}
          </button>
        );
      })}
    </nav>
  );
};
