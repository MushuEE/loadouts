// src/types/inventory.ts

export interface Item {
  id: string;
  name: string;
  description: string;
  weight: number; // in grams
  cost: number; // in cents
  imageUrl: string;
  consumable: boolean;
  slots?: SlotDefinition[];
}

export interface SlotDefinition {
  id: string;
  name: "Worn Torso" | "Worn Legs" | "Worn Feet" | "Carried Pack" | "Hiking Poles" | string; // Example slot names
  capacity: number; // How many items it can hold (usually 1)
  accepts: string[]; // Array of item IDs or categories it can accept
}

export interface LoadoutNode {
  instanceId: string; // Unique ID for this specific instance of an item in the loadout
  itemId: string;
  children: Record<string, LoadoutNode | null>; // Map of SlotDefinition.id -> LoadoutNode
  layout: Record<string, number>; // Map of SlotDefinition.id -> grid index (0-31)
}
