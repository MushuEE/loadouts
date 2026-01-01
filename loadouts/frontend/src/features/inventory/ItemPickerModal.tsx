// src/features/inventory/ItemPickerModal.tsx

import React from 'react';
import { mockDb } from '../../data/mockDb';
import { Item } from '../../types/inventory';

interface ItemPickerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onItemSelect: (itemId: string) => void;
  filter?: string[]; // e.g., ['pack', 'shelter']
}

const ItemPickerModal: React.FC<ItemPickerModalProps> = ({ isOpen, onClose, onItemSelect, filter }) => {
  if (!isOpen) return null;

  const items = Object.values(mockDb).filter(item => !filter || filter.includes(item.id) || filter.some(cat => item.id.includes(cat)));


  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
      <div className="bg-gray-800 p-6 rounded-lg max-w-2xl w-full">
        <h2 className="text-xl font-bold mb-4">Select an Item</h2>
        <div className="grid grid-cols-4 gap-4">
          {items.map(item => (
            <div 
              key={item.id}
              onClick={() => onItemSelect(item.id)}
              className="p-2 bg-gray-700 rounded-md cursor-pointer hover:bg-gray-600 flex flex-col items-center"
            >
              <img src={item.imageUrl} alt={item.name} className="w-16 h-16 mb-2"/>
              <span className="text-sm text-center">{item.name}</span>
            </div>
          ))}
        </div>
        <button onClick={onClose} className="mt-6 bg-red-500 px-4 py-2 rounded">Close</button>
      </div>
    </div>
  );
};

export default ItemPickerModal;
