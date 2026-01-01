// src/data/mockDb.ts

import { Item, LoadoutNode } from '../types/inventory';
import { v4 as uuidv4 } from 'uuid';

export const mockDb: Record<string, Item> = {
  hiker: {
    id: 'hiker',
    name: 'Hiker',
    description: 'The person carrying the gear.',
    weight: 0,
    cost: 0,
    imageUrl: '/icons/hiker.svg',
    consumable: false,
    slots: [
      { id: 'worn-torso', name: 'Worn Torso', capacity: 1, accepts: ['clothing-torso'] },
      { id: 'worn-legs', name: 'Worn Legs', capacity: 1, accepts: ['clothing-legs'] },
      { id: 'worn-feet', name: 'Worn Feet', capacity: 1, accepts: ['footwear'] },
      { id: 'carried-pack', name: 'Carried Pack', capacity: 1, accepts: ['pack'] },
      { id: 'hiking-poles', name: 'Hiking Poles', capacity: 1, accepts: ['poles'] },
    ],
  },
  // Packs
  'hyperlite-southwest-3400': {
    id: 'hyperlite-southwest-3400',
    name: 'Hyperlite Southwest 3400',
    description: 'A durable and lightweight backpack.',
    weight: 900,
    cost: 37500,
    imageUrl: '/gear/pack.png',
    consumable: false,
    slots: [
      { id: 'pack-main', name: 'Main Compartment', capacity: 1, accepts: ['shelter', 'sleep-system', 'kitchen', 'food-bag'] },
    ],
  },
  // Shelter
  'durston-xmid-1': {
    id: 'durston-xmid-1',
    name: 'Durston X-Mid 1',
    description: 'A popular ultralight trekking pole shelter.',
    weight: 795,
    cost: 24000,
    imageUrl: '/gear/shelter.png',
    consumable: false,
  },
  // Sleep System
  'enlightened-equipment-quilt': {
    id: 'enlightened-equipment-quilt',
    name: 'EE Quilt 20°F',
    description: 'A lightweight and warm quilt.',
    weight: 550,
    cost: 35000,
    imageUrl: '/gear/quilt.png',
    consumable: false,
  },
  'thermarest-uberlite': {
    id: 'thermarest-uberlite',
    name: 'Therm-a-Rest UberLite',
    description: 'The lightest insulated sleeping pad.',
    weight: 250,
    cost: 21000,
    imageUrl: '/gear/pad.png',
    consumable: false,
  },
  // Kitchen
  'toaks-pot-750': {
    id: 'toaks-pot-750',
    name: 'Toaks Titanium 750ml Pot',
    description: 'A lightweight titanium pot for cooking.',
    weight: 103,
    cost: 3500,
    imageUrl: '/gear/pot.png',
    consumable: false,
    slots: [
        { id: 'pot-contents', name: 'Pot Contents', capacity: 1, accepts: ['stove', 'fuel'] },
    ]
  },
  'brs-3000t': {
    id: 'brs-3000t',
    name: 'BRS-3000T Stove',
    description: 'A tiny and ultralight canister stove.',
    weight: 25,
    cost: 1700,
    imageUrl: '/gear/stove.png',
    consumable: false,
  },
  // Consumables
  'smartwater-bottle': {
    id: 'smartwater-bottle',
    name: 'SmartWater Bottle 1L',
    description: 'A disposable water bottle, popular with hikers.',
    weight: 38, // Empty weight
    cost: 200,
    imageUrl: '/gear/water.png',
    consumable: true,
  },
  'food-bag': {
    id: 'food-bag',
    name: 'Food Bag',
    description: 'Contains all the food for a trip.',
    weight: 1000, // Example weight with food
    cost: 5000, // Example cost of food
    imageUrl: '/gear/food.png',
    consumable: true,
  },
  // Worn Clothing
  'patagonia-sun-hoody': {
    id: 'patagonia-sun-hoody',
    name: 'Patagonia Sun Hoody',
    description: 'A lightweight sun shirt.',
    weight: 139,
    cost: 6000,
    imageUrl: '/gear/hoody.png',
    consumable: false,
  },
  'altra-lone-peak': {
    id: 'altra-lone-peak',
    name: 'Altra Lone Peak 8',
    description: 'Popular trail running shoes for hiking.',
    weight: 303, // Per shoe
    cost: 14000,
    imageUrl: '/gear/shoes.png',
    consumable: false,
  },
};

export const initialLoadout: LoadoutNode = {
  instanceId: uuidv4(),
  itemId: 'hiker',
  children: {
    'worn-torso': null,
    'worn-legs': null,
    'worn-feet': null,
    'carried-pack': null,
    'hiking-poles': null,
  },
  layout: {},
};
