import React, { useMemo, useState } from 'react';
import { DriverDTO, LoadDTO, MatchDTO } from '../../types/api';
import { Truck, PackageCheck, Zap } from 'lucide-react';

interface NetworkCanvasProps {
  drivers: DriverDTO[];
  loads: LoadDTO[];
  matches: MatchDTO[];
  onSelectMatch?: (match: MatchDTO) => void;
  selectedMatch?: MatchDTO | null;
}

export const NetworkCanvas: React.FC<NetworkCanvasProps> = ({
  drivers,
  loads,
  matches,
  onSelectMatch,
  selectedMatch,
}) => {
  const [hoveredEntity, setHoveredEntity] = useState<string | null>(null);

  // Compute bounding box and normalize coordinates to SVG viewbox (800 x 460)
  const { nodes, projectCoord } = useMemo(() => {
    let minLat = 90, maxLat = -90, minLon = 180, maxLon = -180;

    const allLocs: { id: string; lat: number; lon: number; label: string }[] = [];

    const addLoc = (id: string, lat: number, lon: number, label: string) => {
      if (isNaN(lat) || isNaN(lon)) return;
      minLat = Math.min(minLat, lat);
      maxLat = Math.max(maxLat, lat);
      minLon = Math.min(minLon, lon);
      maxLon = Math.max(maxLon, lon);
      allLocs.push({ id, lat, lon, label });
    };

    drivers.forEach((d) => {
      addLoc(d.current_location.node_id, d.current_location.lat, d.current_location.lon, d.current_location.node_id);
    });

    loads.forEach((l) => {
      addLoc(l.origin.node_id, l.origin.lat, l.origin.lon, l.origin.node_id);
      addLoc(l.destination.node_id, l.destination.lat, l.destination.lon, l.destination.node_id);
    });

    if (allLocs.length === 0) {
      minLat = 25; maxLat = 50; minLon = -125; maxLon = -65;
    }

    const latPad = Math.max(1.0, (maxLat - minLat) * 0.15);
    const lonPad = Math.max(1.0, (maxLon - minLon) * 0.15);

    const effectiveMinLat = minLat - latPad;
    const effectiveMaxLat = maxLat + latPad;
    const effectiveMinLon = minLon - lonPad;
    const effectiveMaxLon = maxLon + lonPad;

    const width = 800;
    const height = 460;

    const project = (lat: number, lon: number) => {
      const x = ((lon - effectiveMinLon) / (effectiveMaxLon - effectiveMinLon || 1)) * (width - 80) + 40;
      const y = height - (((lat - effectiveMinLat) / (effectiveMaxLat - effectiveMinLat || 1)) * (height - 80) + 40);
      return { x, y };
    };

    // Deduplicate nodes
    const nodeMap = new Map<string, { id: string; x: number; y: number; label: string; lat: number; lon: number }>();
    allLocs.forEach((loc) => {
      if (!nodeMap.has(loc.id)) {
        const { x, y } = project(loc.lat, loc.lon);
        nodeMap.set(loc.id, { id: loc.id, x, y, label: loc.label, lat: loc.lat, lon: loc.lon });
      }
    });

    return {
      nodes: Array.from(nodeMap.values()),
      projectCoord: project,
    };
  }, [drivers, loads]);

  const matchLookup = useMemo(() => {
    const map = new Map<string, MatchDTO>();
    matches.forEach((m) => {
      map.set(`${m.driver_id}->${m.load_id}`, m);
      map.set(m.load_id, m);
      map.set(m.driver_id, m);
    });
    return map;
  }, [matches]);

  const loadLookup = useMemo(() => {
    const map = new Map<string, LoadDTO>();
    loads.forEach((l) => map.set(l.id, l));
    return map;
  }, [loads]);

  const driverLookup = useMemo(() => {
    const map = new Map<string, DriverDTO>();
    drivers.forEach((d) => map.set(d.id, d));
    return map;
  }, [drivers]);

  return (
    <div className="relative bg-slate-950/80 rounded-2xl border border-slate-800/80 overflow-hidden shadow-2xl backdrop-blur-md">
      {/* Visual Canvas Header */}
      <div className="px-5 py-3.5 border-b border-slate-800/80 flex items-center justify-between bg-slate-900/60">
        <div className="flex items-center space-x-2">
          <div className="w-2.5 h-2.5 rounded-full bg-cyan-400 animate-pulse"></div>
          <h3 className="text-xs font-bold text-slate-200 uppercase tracking-wider font-mono">
            Spatial Dispatch & Match Topology
          </h3>
          <span className="text-xs text-slate-500 font-mono">
            ({nodes.length} Nodes • {drivers.length} Drivers • {loads.length} Loads)
          </span>
        </div>
        <div className="flex items-center space-x-4 text-[11px] font-mono">
          <div className="flex items-center space-x-1.5 text-slate-400">
            <span className="w-2.5 h-2.5 rounded-full bg-cyan-400 shadow-sm shadow-cyan-400/50"></span>
            <span>Driver Node</span>
          </div>
          <div className="flex items-center space-x-1.5 text-slate-400">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 shadow-sm shadow-emerald-400/50"></span>
            <span>Matched Dispatch</span>
          </div>
          <div className="flex items-center space-x-1.5 text-slate-400">
            <span className="w-2 h-0.5 bg-slate-600 border border-dashed border-slate-500"></span>
            <span>Unmatched Arc</span>
          </div>
        </div>
      </div>

      {/* SVG Map Canvas */}
      <div className="p-2 relative flex items-center justify-center min-h-[460px]">
        <svg viewBox="0 0 800 460" className="w-full h-auto max-h-[500px] select-none">
          <defs>
            <linearGradient id="grad-matched" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#06b6d4" stopOpacity="0.9" />
              <stop offset="100%" stopColor="#10b981" stopOpacity="0.9" />
            </linearGradient>
            <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur stdDeviation="3" result="blur" />
              <feComposite in="SourceGraphic" in2="blur" operator="over" />
            </filter>
            <marker id="arrow-matched" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto">
              <path d="M 0 1 L 10 5 L 0 9 z" fill="#10b981" />
            </marker>
            <marker id="arrow-unmatched" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto">
              <path d="M 0 1 L 10 5 L 0 9 z" fill="#475569" />
            </marker>
          </defs>

          {/* Grid Background Lines */}
          <g opacity="0.06">
            {Array.from({ length: 9 }).map((_, i) => (
              <line key={`gx-${i}`} x1={i * 100} y1="0" x2={i * 100} y2="460" stroke="#38bdf8" strokeWidth="1" />
            ))}
            {Array.from({ length: 6 }).map((_, i) => (
              <line key={`gy-${i}`} x1="0" y1={i * 90} x2="800" y2={i * 90} stroke="#38bdf8" strokeWidth="1" />
            ))}
          </g>

          {/* 1. Unmatched Load Arcs */}
          {loads.map((l) => {
            const isMatched = matchLookup.has(l.id);
            if (isMatched) return null;
            const p1 = projectCoord(l.origin.lat, l.origin.lon);
            const p2 = projectCoord(l.destination.lat, l.destination.lon);
            return (
              <line
                key={`unmatched-${l.id}`}
                x1={p1.x}
                y1={p1.y}
                x2={p2.x}
                y2={p2.y}
                stroke="#334155"
                strokeWidth="1.5"
                strokeDasharray="4,4"
                markerEnd="url(#arrow-unmatched)"
                opacity="0.6"
              />
            );
          })}

          {/* 2. Matched Assignment Arcs & Deadhead Paths */}
          {matches.map((m) => {
            const load = loadLookup.get(m.load_id);
            const driver = driverLookup.get(m.driver_id);
            if (!load || !driver) return null;

            const pD = projectCoord(driver.current_location.lat, driver.current_location.lon);
            const pO = projectCoord(load.origin.lat, load.origin.lon);
            const pDest = projectCoord(load.destination.lat, load.destination.lon);

            const isSelected = selectedMatch?.load_id === m.load_id && selectedMatch?.driver_id === m.driver_id;

            return (
              <g
                key={`match-arc-${m.driver_id}-${m.load_id}`}
                className="cursor-pointer transition-all"
                onClick={() => onSelectMatch && onSelectMatch(m)}
                onMouseEnter={() => setHoveredEntity(`match-${m.driver_id}-${m.load_id}`)}
                onMouseLeave={() => setHoveredEntity(null)}
              >
                {/* Deadhead leg (Driver -> Pickup) */}
                <line
                  x1={pD.x}
                  y1={pD.y}
                  x2={pO.x}
                  y2={pO.y}
                  stroke="#38bdf8"
                  strokeWidth={isSelected ? '2.5' : '1.5'}
                  strokeDasharray="3,3"
                  opacity={isSelected ? 1 : 0.75}
                />

                {/* Loaded Linehaul leg (Pickup -> Destination) */}
                <line
                  x1={pO.x}
                  y1={pO.y}
                  x2={pDest.x}
                  y2={pDest.y}
                  stroke="url(#grad-matched)"
                  strokeWidth={isSelected ? '4' : '2.5'}
                  filter={isSelected ? 'url(#glow)' : undefined}
                  markerEnd="url(#arrow-matched)"
                  opacity={isSelected ? 1 : 0.9}
                />
              </g>
            );
          })}

          {/* 3. Physical Network Spatial Nodes */}
          {nodes.map((n) => {
            const isHovered = hoveredEntity === n.id;
            return (
              <g key={`node-${n.id}`} className="transition-all">
                <circle
                  cx={n.x}
                  y={n.y}
                  r={isHovered ? '7' : '5'}
                  fill="#0f172a"
                  stroke="#38bdf8"
                  strokeWidth="2"
                  className="shadow-md"
                />
                <text
                  x={n.x}
                  y={n.y - 8}
                  textAnchor="middle"
                  fill="#94a3b8"
                  fontSize="10"
                  fontFamily="JetBrains Mono, monospace"
                  fontWeight="600"
                  className="pointer-events-none select-none"
                >
                  {n.label}
                </text>
              </g>
            );
          })}
        </svg>

        {/* Floating Quick Stats Overlay */}
        <div className="absolute bottom-4 left-4 bg-slate-900/90 border border-slate-800 px-3.5 py-2 rounded-xl text-xs font-mono backdrop-blur-md shadow-xl flex items-center space-x-3">
          <div className="flex items-center space-x-1.5 text-cyan-400">
            <Truck className="w-3.5 h-3.5" />
            <span>Active Drivers: {drivers.length}</span>
          </div>
          <span className="text-slate-600">|</span>
          <div className="flex items-center space-x-1.5 text-emerald-400">
            <PackageCheck className="w-3.5 h-3.5" />
            <span>Matched: {matches.length}</span>
          </div>
          <span className="text-slate-600">|</span>
          <div className="flex items-center space-x-1.5 text-amber-400">
            <Zap className="w-3.5 h-3.5" />
            <span>Unmatched Freight: {Math.max(0, loads.length - matches.length)}</span>
          </div>
        </div>
      </div>
    </div>
  );
};
