import { useState } from 'react';
import { Compass } from 'lucide-react';
import { api } from '../api/client';
import type { Community, GearSearch, LoadoutSummary, TagCount } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { useGearScope } from '../lib/gearScope';
import { CORE, formatCost, formatGrams } from '../lib/display';
import { useSession } from '../session/SessionContext';
import { Badge, EmptyState, ErrorNote, LoadoutCard, Spinner } from '../components/ui';
import { ItemThumb } from '../components/ItemThumb';
import { ScopeToggle } from '../components/ScopeToggle';
import { NO_TERMS, ResultTags, SearchBar, TagSuggestions, type SearchTerms } from '../components/GearSearchControls';

type Kind = 'loadouts' | 'items';

/**
 * Discover is the Lurker persona's entry point: browse published loadouts and the gear in
 * them, filter by community, tag or category, and fork anything into your own account.
 *
 * The scope switch turns the same search on your own things: your loadouts (drafts
 * included) and your gear, matched against your own tags.
 */
export function DiscoverView({ onOpenLoadout }: { onOpenLoadout: (id: string) => void }) {
  const [kind, setKind] = useState<Kind>('loadouts');
  const [terms, setTerms] = useState<SearchTerms>(NO_TERMS);
  const [scope] = useGearScope();
  const mine = scope === 'mine';

  return (
    <div className="flex-1 overflow-y-auto bg-stone-950 text-white p-10">
      <div className="max-w-5xl mx-auto">
        <div className="flex items-center gap-3 mb-2">
          <Compass className="w-6 h-6 text-orange-500" />
          <h1 className="text-3xl font-light">Discover</h1>
          <div className="ml-auto">
            <ScopeToggle />
          </div>
        </div>
        <p className="text-stone-500 text-sm mb-6">
          {mine
            ? 'Your own loadouts and gear, drafts included. Tags mean your tags.'
            : 'Published loadouts and gear from everyone. Fork a loadout to make it yours.'}
        </p>

        <div className="flex gap-1 mb-4 border-b border-stone-800">
          {(['loadouts', 'items'] as Kind[]).map((k) => (
            <button
              key={k}
              onClick={() => setKind(k)}
              className={`px-4 py-2 text-sm -mb-px border-b-2 transition-colors ${
                kind === k ? 'border-orange-500 text-white' : 'border-transparent text-stone-500 hover:text-stone-300'
              }`}
            >
              {k === 'loadouts' ? 'Loadouts' : 'Gear'}
            </button>
          ))}
        </div>

        {kind === 'loadouts' ? (
          <LoadoutResults terms={terms} setTerms={setTerms} onOpenLoadout={onOpenLoadout} />
        ) : (
          <GearResults terms={terms} setTerms={setTerms} />
        )}
      </div>
    </div>
  );
}

function LoadoutResults({
  terms,
  setTerms,
  onOpenLoadout,
}: {
  terms: SearchTerms;
  setTerms: (t: SearchTerms) => void;
  onOpenLoadout: (id: string) => void;
}) {
  const session = useSession();
  const [scope] = useGearScope();
  const [communityId, setCommunityId] = useState('');
  const [notice, setNotice] = useState<string | null>(null);

  const communities = useAsync<Community[]>(() => api.listCommunities(), []);
  const popular = useAsync<TagCount[]>(() => api.popularTags('', 12).then((r) => r.tags), []);
  const feed = useAsync<LoadoutSummary[]>(
    () => api.discover({ q: terms.q, tags: terms.tags, community_id: communityId, scope }),
    [terms.q, terms.tags.join(','), communityId, scope, session.profile?.id],
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
    <>
      <div className="flex flex-wrap gap-3 mb-3">
        <SearchBar terms={terms} onChange={setTerms} placeholder="Search loadouts, or #tag…" />
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
      <div className="mb-6">
        <TagSuggestions suggestions={popular.data ?? []} terms={terms} onChange={setTerms} unit="people use this tag" />
      </div>

      {notice && <div className="mb-4 text-sm text-orange-300">{notice}</div>}
      {feed.loading && !feed.data && <Spinner />}
      {feed.error && <ErrorNote message={feed.error} />}
      {feed.data && feed.data.length === 0 && (
        <EmptyState
          title={terms.tags.length || terms.q ? 'No loadouts match' : scope === 'mine' ? 'You have no loadouts yet' : 'Nothing published yet'}
          hint={terms.tags.length ? 'A loadout matches a tag when its owner tagged gear in it.' : undefined}
        />
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
    </>
  );
}

function GearResults({ terms, setTerms }: { terms: SearchTerms; setTerms: (t: SearchTerms) => void }) {
  const session = useSession();
  const [scope] = useGearScope();
  const [category, setCategory] = useState('');

  const search = useAsync<GearSearch>(
    () => api.searchGear({ q: terms.q, tags: terms.tags, category, scope }),
    [terms.q, terms.tags.join(','), category, scope, session.profile?.id],
  );
  const data = search.data;
  const addTag = (tag: string) => !terms.tags.includes(tag) && setTerms({ ...terms, tags: [...terms.tags, tag] });

  return (
    <>
      <div className="flex flex-wrap gap-3 mb-3">
        <SearchBar terms={terms} onChange={setTerms} placeholder="Search gear, or #tag…" />
      </div>
      {data && (
        <div className="flex flex-wrap gap-1.5 mb-3">
          <CategoryChip label="All" active={category === ''} onClick={() => setCategory('')} />
          {data.category_facets.map((c) => (
            <CategoryChip
              key={c.category}
              label={c.category}
              count={c.count}
              active={category === c.category}
              onClick={() => setCategory(category === c.category ? '' : c.category)}
            />
          ))}
        </div>
      )}
      <div className="mb-6">
        <TagSuggestions
          suggestions={data?.tag_facets ?? []}
          terms={terms}
          onChange={setTerms}
          unit={scope === 'mine' ? 'of your results you tagged this' : 'results carry this tag'}
        />
      </div>

      {search.loading && !data && <Spinner />}
      {search.error && <ErrorNote message={search.error} />}
      {data && data.results.length === 0 && (
        <EmptyState
          title="No gear matches"
          hint={
            scope === 'mine'
              ? 'Your gear is what you use in your loadouts, tag, annotate or import. Try Everyone.'
              : undefined
          }
        />
      )}
      {data && data.results.length > 0 && (
        <div className="text-[11px] text-stone-500 mb-2 tabular-nums">
          {data.total} {data.total === 1 ? 'item' : 'items'}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
        {data?.results.map((r) => (
          <div key={r.item.id} className="flex items-start gap-3 p-3 rounded-xl border border-stone-800 bg-stone-900/40">
            <ItemThumb imageUrl={r.item.image_url} category={r.item.category} size="w-12 h-12" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm text-stone-200 truncate">{r.item.name}</span>
                {r.mine && scope === 'everyone' && (
                  <Badge className="bg-orange-500/15 text-orange-300" title="In your gear">
                    yours
                  </Badge>
                )}
              </div>
              <div className="text-[10px] uppercase tracking-wider text-stone-600 mb-1.5">
                {r.item.category} · {formatGrams(Number(r.item.base_metadata?.[CORE]?.weight_g ?? 0))} ·{' '}
                {formatCost(Number(r.item.base_metadata?.[CORE]?.cost_cents ?? 0))}
              </div>
              <ResultTags result={r} scope={scope} onTag={addTag} />
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

function CategoryChip({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      aria-pressed={active}
      className={`px-2.5 py-1 rounded-md text-xs capitalize border transition-colors ${
        active ? 'bg-stone-100 text-stone-900 border-stone-100' : 'border-stone-800 text-stone-400 hover:text-white hover:border-stone-600'
      }`}
    >
      {label}
      {count !== undefined && <span className={`ml-1.5 tabular-nums ${active ? 'text-stone-500' : 'text-stone-600'}`}>{count}</span>}
    </button>
  );
}
