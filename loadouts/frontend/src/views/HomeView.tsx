import { useState } from 'react';
import { Plus } from 'lucide-react';
import { api } from '../api/client';
import type { Community, LoadoutSummary } from '../api/types';
import { useSession } from '../session/SessionContext';
import { useAsync } from '../lib/useAsync';
import { EmptyState, ErrorNote, LoadoutCard, Spinner } from '../components/ui';
import { NewLoadoutModal } from '../components/NewLoadoutModal';

/** Home is the acting profile's shelf: everything they own, drafts included. */
export function HomeView({ onOpenLoadout }: { onOpenLoadout: (id: string) => void }) {
  const { profile } = useSession();
  const [creating, setCreating] = useState(false);

  const loadouts = useAsync<LoadoutSummary[]>(() => api.myLoadouts(), [profile?.id]);
  const communities = useAsync<Community[]>(
    () => (profile ? api.profileCommunities(profile.handle) : Promise.resolve([])),
    [profile?.id],
  );

  return (
    <div className="flex-1 overflow-y-auto bg-stone-950 text-white p-10">
      <div className="max-w-5xl mx-auto">
        <div className="flex justify-between items-end mb-8">
          <div>
            <h1 className="text-3xl font-light">
              {profile ? profile.display_name : 'Loadouts'}
            </h1>
            <p className="text-stone-500 text-sm mt-1">
              {profile ? `@${profile.handle} · ${profile.bio || 'No bio yet.'}` : 'Pick a profile to get started.'}
            </p>
          </div>
          <button
            onClick={() => setCreating(true)}
            className="flex items-center gap-2 px-4 py-2 bg-orange-500 hover:bg-orange-400 rounded-lg text-sm font-medium"
          >
            <Plus className="w-4 h-4" /> New Loadout
          </button>
        </div>

        {communities.data && communities.data.length > 0 && (
          <div className="mb-8 flex flex-wrap gap-2">
            {communities.data.map((c) => (
              <span key={c.id} className="px-3 py-1 rounded-full bg-stone-900 border border-stone-800 text-xs text-stone-400">
                {c.name}
              </span>
            ))}
          </div>
        )}

        {loadouts.loading && <Spinner />}
        {loadouts.error && <ErrorNote message={loadouts.error} />}

        {loadouts.data && loadouts.data.length === 0 && (
          <EmptyState title="No loadouts yet" hint="Start from a template, or fork one from Discover." />
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {loadouts.data?.map((summary) => (
            <LoadoutCard
              key={summary.loadout.id}
              summary={summary}
              showOwner={false}
              onOpen={() => onOpenLoadout(summary.loadout.id)}
            />
          ))}
        </div>
      </div>

      {creating && (
        <NewLoadoutModal
          communities={communities.data ?? []}
          onClose={() => setCreating(false)}
          onCreated={(id) => {
            setCreating(false);
            loadouts.reload();
            onOpenLoadout(id);
          }}
        />
      )}
    </div>
  );
}
