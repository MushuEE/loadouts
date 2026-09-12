import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, Bookmark, BookmarkCheck, RefreshCw, ShieldCheck } from 'lucide-react';
import { api } from '../api/client';
import type { Community, FavoriteView } from '../api/types';
import { useSession } from '../session/SessionContext';
import { Badge } from './ui';

/**
 * Endorsement controls for one loadout.
 *
 * Two different things live here on purpose, because they read as one gesture to a user
 * but mean very different things:
 *
 *  - A private bookmark ("save this"), which nobody else ever sees.
 *  - A community endorsement ("our community vouches for this"), which is public and
 *    takes an admin.
 *
 * Neither gives anyone any control over the loadout. A community that wants to change an
 * endorsed kit forks it, exactly like anyone else.
 */
export function FavoritePanel({ loadoutId }: { loadoutId: string }) {
  const { profile } = useSession();
  const [bookmark, setBookmark] = useState<FavoriteView | null>(null);
  const [endorsements, setEndorsements] = useState<FavoriteView[]>([]);
  const [adminOf, setAdminOf] = useState<Community[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [mine, public_] = await Promise.all([
        profile ? api.listFavorites({ scope_type: 'profile', scope_id: profile.id }) : Promise.resolve([]),
        api.listLoadoutFavorites(loadoutId),
      ]);
      setBookmark(mine.find((f) => f.loadout_id === loadoutId) ?? null);
      setEndorsements(public_);
    } catch (err) {
      setError((err as Error).message);
    }
  }, [loadoutId, profile]);

  useEffect(() => {
    void load();
  }, [load]);

  // Which communities may this profile endorse on behalf of? The membership list does not
  // carry a role, so each one has to be asked. It is a handful of requests for a handful
  // of communities, and it runs once per loadout view.
  useEffect(() => {
    if (!profile) {
      setAdminOf([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const communities = await api.profileCommunities(profile.handle);
        const views = await Promise.all(
          communities.map((c) => api.getCommunity(c.slug).catch(() => null)),
        );
        if (cancelled) return;
        setAdminOf(
          views
            .filter((v) => v && (v.viewer_role === 'owner' || v.viewer_role === 'admin'))
            .map((v) => v!.community),
        );
      } catch {
        if (!cancelled) setAdminOf([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [profile]);

  async function act(fn: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
      await load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!profile && endorsements.length === 0) return null;

  return (
    <div className="px-6 pb-6 space-y-3">
      <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500">Endorsements</div>

      {error && <div className="text-[11px] text-red-400">{error}</div>}

      {profile && (
        <button
          disabled={busy}
          onClick={() =>
            act(() =>
              bookmark
                ? api.unfavoriteLoadout(loadoutId, { scope_type: 'profile', scope_id: profile.id })
                : api.favoriteLoadout(loadoutId),
            )
          }
          className={`w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 ${
            bookmark
              ? 'bg-orange-500/15 text-orange-300 border border-orange-500/30'
              : 'bg-stone-800 text-stone-400 border border-stone-700 hover:text-white'
          }`}
        >
          {bookmark ? <BookmarkCheck size={14} /> : <Bookmark size={14} />}
          {bookmark ? 'Saved' : 'Save'}
        </button>
      )}

      {endorsements.map((e) => (
        <EndorsementRow
          key={e.id}
          endorsement={e}
          canManage={adminOf.some((c) => c.id === e.scope_id)}
          busy={busy}
          onReconfirm={() =>
            act(() => api.favoriteLoadout(loadoutId, { scope_type: 'community', scope_id: e.scope_id }))
          }
          onWithdraw={() =>
            act(() => api.unfavoriteLoadout(loadoutId, { scope_type: 'community', scope_id: e.scope_id }))
          }
        />
      ))}

      {adminOf
        .filter((c) => !endorsements.some((e) => e.scope_id === c.id))
        .map((c) => (
          <button
            key={c.id}
            disabled={busy}
            onClick={() =>
              act(() => api.favoriteLoadout(loadoutId, { scope_type: 'community', scope_id: c.id }))
            }
            className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs bg-stone-800/60 text-stone-400 border border-dashed border-stone-700 hover:text-white hover:border-stone-600 transition-colors disabled:opacity-50"
          >
            <ShieldCheck size={14} />
            Endorse for {c.name}
          </button>
        ))}
    </div>
  );
}

function EndorsementRow({
  endorsement,
  canManage,
  busy,
  onReconfirm,
  onWithdraw,
}: {
  endorsement: FavoriteView;
  canManage: boolean;
  busy: boolean;
  onReconfirm: () => void;
  onWithdraw: () => void;
}) {
  return (
    <div className="p-3 rounded-lg bg-stone-800 border border-stone-700 space-y-2">
      <div className="flex items-start gap-2">
        <ShieldCheck size={14} className="text-emerald-400 mt-0.5 shrink-0" />
        <div className="min-w-0 flex-1">
          <div className="text-xs text-stone-200 truncate">
            Endorsed by {endorsement.scope_name || 'a community'}
          </div>
          {endorsement.note && (
            <div className="text-[11px] text-stone-500 mt-0.5">“{endorsement.note}”</div>
          )}
        </div>
      </div>

      {/* The whole point of the fingerprint: an endorsement that no longer describes what
          it endorsed says so, instead of silently vouching for something else. */}
      {endorsement.stale && (
        <div className="flex items-start gap-2">
          <AlertTriangle size={12} className="text-amber-400 mt-0.5 shrink-0" />
          <Badge
            className="bg-amber-500/15 text-amber-300"
            title="The owner changed this loadout's name or contents after it was endorsed."
          >
            Edited since endorsed
          </Badge>
        </div>
      )}

      {canManage && (
        <div className="flex gap-2">
          {endorsement.stale && (
            <button
              disabled={busy}
              onClick={onReconfirm}
              className="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25 disabled:opacity-50"
            >
              <RefreshCw size={10} /> Re-confirm
            </button>
          )}
          <button
            disabled={busy}
            onClick={onWithdraw}
            className="px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider text-stone-500 hover:text-red-400 disabled:opacity-50"
          >
            Withdraw
          </button>
        </div>
      )}
    </div>
  );
}
