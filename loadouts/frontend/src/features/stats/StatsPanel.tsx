// src/features/stats/StatsPanel.tsx

import React from 'react';

interface StatsPanelProps {
  baseWeight: number;
  totalWeight: number;
}

const StatsPanel: React.FC<StatsPanelProps> = ({ baseWeight, totalWeight }) => {
  return (
    <div className="p-4 bg-gray-800 text-white rounded-lg">
      <h2 className="text-xl font-bold mb-4">Loadout Stats</h2>
      <div className="space-y-2">
        <div>
          <span className="font-semibold">Base Weight:</span>
          <span className="float-right">{(baseWeight / 1000).toFixed(2)} kg</span>
        </div>
        <div>
          <span className="font-semibold">Total Weight:</span>
          <span className="float-right">{(totalWeight / 1000).toFixed(2)} kg</span>
        </div>
      </div>
    </div>
  );
};

export default StatsPanel;
