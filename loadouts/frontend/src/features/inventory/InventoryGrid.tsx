// src/features/inventory/InventoryGrid.tsx

import React from 'react';
import { LoadoutNode } from '../../types/inventory';
import { mockDb } from '../../data/mockDb';
import { PlusCircle, ArrowRight } from 'lucide-react';

interface InventoryGridProps {
  node: LoadoutNode;
  onItemAdd: (slotId: string) => void;
  onItemZoom: (instanceId: string) => void;
}

const InventoryGrid: React.FC<InventoryGridProps> = ({ node, onItemAdd, onItemZoom }) => {
  const item = mockDb[node.itemId];
  const slots = item.slots || [];

  const occupiedIndices = new Set(Object.values(node.layout));

  const getNextAvailableIndex = () => {
    for (let i = 0; i < 32; i++) {
      if (!occupiedIndices.has(i)) {
        return i;
      }
    }
    return -1; // No space left
  };

  const renderSlot = (slotDef: any, index: number) => {
    const childNode = node.children[slotDef.id];
    
    if (childNode) {
      const childItem = mockDb[childNode.itemId];
      return (
        <div 
          key={slotDef.id}
          className="bg-gray-700 border border-gray-600 rounded-md p-2 flex flex-col items-center justify-center"
          style={{ gridColumn: `${(index % 4) + 1}`, gridRow: `${Math.floor(index / 4) + 1}`}}
        >
          <img src={childItem.imageUrl} alt={childItem.name} className="w-12 h-12 mb-2"/>
          <span className="text-xs text-center">{childItem.name}</span>
          {childItem.slots && (
             <button onClick={() => onItemZoom(childNode.instanceId)} className="mt-2 text-blue-400">
                <ArrowRight size={16} />
             </button>
          )}
        </div>
      );
    }

    return (
      <div 
        key={slotDef.id}
        className="border border-dashed border-gray-600 rounded-md flex items-center justify-center"
        style={{ gridColumn: `${(index % 4) + 1}`, gridRow: `${Math.floor(index / 4) + 1}`}}
      >
        <button onClick={() => onItemAdd(slotDef.id)} className="text-gray-400 hover:text-white">
          <PlusCircle size={32} />
          <span className="text-xs">{slotDef.name}</span>
        </button>
      </div>
    );
  };
  
  return (
    <div className="grid grid-cols-4 grid-rows-8 gap-2 p-4 bg-gray-900 rounded-lg h-[600px]">
      {slots.map((slot) => {
        const layoutIndex = node.layout[slot.id];
        const index = layoutIndex !== undefined ? layoutIndex : getNextAvailableIndex();
        if (index !== -1 && layoutIndex === undefined) {
            node.layout[slot.id] = index; // Mutating prop, but acceptable for this demo's auto-layout
            occupiedIndices.add(index);
        }
        return renderSlot(slot, index);
      })}
    </div>
  );
};

export default InventoryGrid;
