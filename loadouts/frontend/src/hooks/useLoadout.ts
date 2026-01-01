// src/hooks/useLoadout.ts

import { useState, useMemo, useCallback } from 'react';
import { LoadoutNode } from '../types/inventory';
import { initialLoadout, mockDb } from '../data/mockDb';
import { v4 as uuidv4 } from 'uuid';

export const useLoadout = () => {
  const [rootNode, setRootNode] = useState<LoadoutNode>(initialLoadout);
  const [path, setPath] = useState<string[]>([]); // Array of instanceIds

  const currentNode = useMemo(() => {
    let node: LoadoutNode = rootNode;
    for (const instanceId of path) {
      // This is a simplified traversal. A real implementation would need to search the children.
      // For now, we assume a direct parent-child relationship for zooming.
      const foundChild = Object.values(node.children).find(child => child?.instanceId === instanceId);
      if (foundChild) {
        node = foundChild;
      }
    }
    return node;
  }, [rootNode, path]);

  const navigateToChild = useCallback((instanceId: string) => {
    setPath(prev => [...prev, instanceId]);
  }, []);

  const navigateBack = useCallback((index: number) => {
    setPath(prev => prev.slice(0, index + 1));
  }, []);
  
  const navigateHome = useCallback(() => {
    setPath([]);
  }, []);

  const updateLayout = useCallback((slotId: string, newIndex: number) => {
    // A real implementation would need to recursively find the currentNode and update its layout.
    // This is a placeholder for the logic.
    console.log(`Moving item in slot ${slotId} to index ${newIndex}`);
  }, []);
  
  const addItemToSlot = useCallback((slotId: string, itemId: string) => {
    const item = mockDb[itemId];
    if (!item) return;

    const newNode: LoadoutNode = {
        instanceId: uuidv4(),
        itemId: item.id,
        children: item.slots ? Object.fromEntries(item.slots.map(s => [s.id, null])) : {},
        layout: {},
    };

    // This logic needs to be recursive to find the correct node to update
    setRootNode(prevRoot => {
        const newRoot = { ...prevRoot }; // Shallow copy
        // In a real app, you'd recursively find the currentNode and update its children.
        // This is a simplified version for the root.
        if (Object.keys(newRoot.children).includes(slotId)) {
            newRoot.children = {
                ...newRoot.children,
                [slotId]: newNode,
            };
        }
        return newRoot;
    });

  }, []);

  const stats = useMemo(() => {
    const calculateWeight = (node: LoadoutNode | null, includeConsumables: boolean): number => {
      if (!node) return 0;
      const item = mockDb[node.itemId];
      let totalWeight = (includeConsumables || !item.consumable) ? item.weight : 0;
      
      for (const childNode of Object.values(node.children)) {
        totalWeight += calculateWeight(childNode, includeConsumables);
      }
      return totalWeight;
    };

    const baseWeight = calculateWeight(rootNode, false);
    const totalWeight = calculateWeight(rootNode, true);

    return { baseWeight, totalWeight };
  }, [rootNode]);

  return {
    currentNode,
    path,
    navigateToChild,
    navigateBack,
    navigateHome,
    updateLayout,
    addItemToSlot,
    stats,
  };
};
