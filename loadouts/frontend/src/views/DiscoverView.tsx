import { useState } from 'react';
import { Compass, Search } from 'lucide-react';
import { api } from '../api/client';
import type { Community, LoadoutSummary } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { EmptyState, ErrorNote, LoadoutCard, Spinner } from '../components/ui';

/**
 * Discover is the Lurker persona's entry point: browse published loadouts, filter by
 * community, and fork anything into your own account.
 */
export function DiscoverView({ onOpenLoadout }: { onOpenLoadout: (id: string) => void }) {
  const [query, setQuery] = useState('');
  const [submitted, setSubmitted] = useState('');
  const [communityId, setCommunityId] = useState('');
  const [notice, setNotice] = useState<string | null>(null);

  const communities = useAsync<Community[]>(() => api.listCommunities(), []);
  const feed = useAsync<LoadoutSummary[]>(
    () => api.discover({ q: submitted, community_id: communityId }),
    [submitted, communityId],
  );

  async function fork(id: string) {
    setNotice(null);
    try {
      const detail = await api.forkLoadout(id);
      setNotice(`Forked into your account as "${detail.loadout.name}".`);
      onOpenLoadout(detail.loadout.id);
    } catch (err) {
      setNotice((err as Error).message);
    }
  }

  return (
    <div className="flex-1 overflow-y-auto bg-stone-950 text-white p-10">
      <div className="max-w-5xl mx-auto">
        <div className="flex items-center gap-3 mb-2">
          <Compass className="w-6 h-6 text-orange-500" />
          <h1 className="text-3xl font-light">Discover</h1>
        </div>
        <p className="text-stone-500 text-sm mb-8">
          Published loadouts from every community. Fork one to make it yours.
        </p>

        <div className="flex flex-wrap gap-3 mb-6">
          <div className="flex items-center bg-stone-900 border border-stone-800 rounded-lg px-3 focus-within:border-orange-500 flex-1 min-w-[240px]">
            <Search className="w-4 h-4 text-stone-500" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && setSubmitted(query)}
              placeholder="Search loadouts…"
              className="bg-transparent px-3 py-2 text-sm text-white outline-none flex-1 placeholder:text-stone-600"
            />
          </div>
          <select
            value={communityId}
            onChange={(e) => setCommunityId(e.target.value)}
            className="bg-stone-900 border border-stone-800 rounded-lg px-3 py-2 text-sm text-stone-300 outline-none focus:border-orange-500"
          >
            <option value="">All communities</option>
            {communities.data?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </div>

        {notice && <div className="mb-4 text-sm text-orange-300">{notice}</div>}
        {feed.loading && <Spinner />}
        {feed.error && <ErrorNote message={feed.error} />}
        {feed.data && feed.data.length === 0 && (
          <EmptyState title="Nothing published yet" hint="Publish one of your own loadouts to seed the feed." />
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {feed.data?.map((summary) => (
            <LoadoutCard
              key={summary.loadout.id}
              summary={summary}
              onOpen={() => onOpenLoadout(summary.loadout.id)}
              onFork={() => fork(summary.loadout.id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
