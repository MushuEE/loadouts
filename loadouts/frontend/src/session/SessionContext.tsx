import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { api, setActingProfile } from '../api/client';
import type { Profile } from '../api/types';

/**
 * Day 0 session: a User owns many Profiles, and the whole app acts as exactly one Profile
 * at a time. Switching profiles is the closest thing we have to logging in, and it changes
 * every downstream read (which loadouts you own, which private item notes you can see).
 */
interface SessionValue {
  profiles: Profile[];
  profile: Profile | null;
  loading: boolean;
  error: string | null;
  switchProfile: (profileId: string) => void;
  reload: () => void;
}

const STORAGE_KEY = 'loadouts.activeProfileId';

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [activeId, setActiveId] = useState<string>(() => localStorage.getItem(STORAGE_KEY) ?? '');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    api
      .listProfiles()
      .then((list) => {
        if (cancelled) return;
        setProfiles(list);
        setError(null);
        // Fall back to the first profile so a fresh browser is immediately usable.
        setActiveId((current) => (list.some((p) => p.id === current) ? current : list[0]?.id ?? ''));
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [nonce]);

  // Keep the API client's auth header in lockstep with the selected profile.
  useEffect(() => {
    setActingProfile(activeId);
    if (activeId) localStorage.setItem(STORAGE_KEY, activeId);
  }, [activeId]);

  const switchProfile = useCallback((profileId: string) => {
    setActingProfile(profileId);
    setActiveId(profileId);
  }, []);

  const value = useMemo<SessionValue>(
    () => ({
      profiles,
      profile: profiles.find((p) => p.id === activeId) ?? null,
      loading,
      error,
      switchProfile,
      reload: () => setNonce((n) => n + 1),
    }),
    [profiles, activeId, loading, error, switchProfile],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error('useSession must be used inside a SessionProvider');
  return ctx;
}
