import React, { createContext, useContext, useEffect, useState } from 'react';
import type { GearScope } from '../api/types';

/**
 * Whose gear a search covers: yours, or everyone's. One setting for the whole app, so
 * flipping it in Discover also flips the Garage and the item picker, and it survives a
 * reload. "Mine" also means *your* tags: #springseattle26 matches what you tagged, where
 * under "everyone" it matches what anybody did.
 */
const STORAGE_KEY = 'loadouts.gearScope';

const GearScopeContext = createContext<[GearScope, (s: GearScope) => void] | null>(null);

export function GearScopeProvider({ children }: { children: React.ReactNode }) {
  const [scope, setScope] = useState<GearScope>(() =>
    localStorage.getItem(STORAGE_KEY) === 'mine' ? 'mine' : 'everyone',
  );
  useEffect(() => localStorage.setItem(STORAGE_KEY, scope), [scope]);
  return <GearScopeContext.Provider value={[scope, setScope]}>{children}</GearScopeContext.Provider>;
}

export function useGearScope(): [GearScope, (s: GearScope) => void] {
  const ctx = useContext(GearScopeContext);
  if (!ctx) throw new Error('useGearScope must be used inside GearScopeProvider');
  return ctx;
}
