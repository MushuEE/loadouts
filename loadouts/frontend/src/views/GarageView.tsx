import { useState } from 'react';
import { PackagePlus, X } from 'lucide-react';
import { api } from '../api/client';
import type { Community, GearSearch, ProfileItemLayer, ResolvedItem } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { useGearScope } from '../lib/gearScope';
import { useSession } from '../session/SessionContext';
import { ScopeToggle } from '../components/ScopeToggle';
import { NO_TERMS, ResultTags, SearchBar, TagSuggestions, type SearchTerms } from '../components/GearSearchControls';
import { CORE, LAYER_STYLES, formatCost, formatGrams } from '../lib/display';
import { ItemThumb } from '../components/ItemThumb';
import { Badge, EmptyState, ErrorNote, Spinner } from '../components/ui';
import { ImportItemModal } from '../components/ImportItemModal';
import { PluginSurfaceHost } from '../components/plugins/PluginSurfaceHost';

/**
 * The Garage is the item catalogue plus your own layer on top of it. The scope switch
 * narrows it to your gear, which is where you go to tidy up what you own.
 */
export function GarageView() {
  const session = useSession();
  const [scope] = useGearScope();
  const [terms, setTerms] = useState<SearchTerms>(NO_TERMS);
  const [selected, setSelected] = useState<string | null>(null);
  const [communitySlug, setCommunitySlug] = useState('');
  const [importing, setImporting] = useState(false);

  const search = useAsync<GearSearch>(
    () => api.searchGear({ q: terms.q, tags: terms.tags, scope }),
    [terms.q, terms.tags.join(','), scope, session.profile?.id],
  );
  const results = search.data?.results;
  const addTag = (tag: string) => !terms.tags.includes(tag) && setTerms({ ...terms, tags: [...terms.tags, tag] });
  const communities = useAsync<Community[]>(() => api.listCommunities(), []);

  return (
    <div className="flex-1 flex overflow-hidden bg-stone-950 text-white">
      <div className="flex-1 flex flex-col overflow-hidden p-8">
        <div className="flex justify-between items-start gap-6 mb-6">
          <div>
            <h1 className="text-3xl font-light">Gear Garage</h1>
            <p className="text-stone-500 text-sm mt-1">
              Global items are shared and immutable. Your edits live in your own layer.
            </p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <ScopeToggle />
            <button
              onClick={() => setImporting(true)}
              className="px-4 py-2 bg-orange-600 hover:bg-orange-500 text-white text-sm font-medium rounded-lg flex items-center gap-2 shrink-0"
            >
              <PackagePlus className="w-4 h-4" />
              Import from a store
            </button>
          </div>
        </div>

        <div className="flex gap-3 mb-3">
          <SearchBar terms={terms} onChange={setTerms} placeholder="Search gear, or #tag…" />
          <select
            value={communitySlug}
            onChange={(e) => setCommunitySlug(e.target.value)}
            title="View items through a community's metadata layer"
            className="bg-stone-900 border border-stone-800 rounded-lg px-3 py-2 text-sm text-stone-300 outline-none focus:border-orange-500"
          >
            <option value="">No community lens</option>
            {communities.data?.map((c) => (
              <option key={c.id} value={c.slug}>
                Lens: {c.name}
              </option>
            ))}
          </select>
        </div>

        <div className="mb-4">
          <TagSuggestions
            suggestions={search.data?.tag_facets ?? []}
            terms={terms}
            onChange={setTerms}
            unit={scope === 'mine' ? 'of your items you tagged this' : 'items carry this tag'}
          />
        </div>

        {search.loading && !search.data && <Spinner />}
        {search.error && <ErrorNote message={search.error} />}
        {results?.length === 0 && (
          <EmptyState
            title={scope === 'mine' ? 'None of your gear matches' : 'No items match that search'}
            hint={scope === 'mine' ? 'Your gear is what you use in your loadouts, tag, annotate or import.' : undefined}
          />
        )}

        <div className="flex-1 overflow-y-auto rounded-xl border border-stone-800 bg-stone-900/30">
          <table className="w-full text-left text-sm">
            <thead className="bg-stone-900 text-stone-500 uppercase font-bold text-[10px] tracking-wider sticky top-0">
              <tr>
                <th className="p-4">Item</th>
                <th className="p-4">Category</th>
                <th className="p-4">Tags</th>
                <th className="p-4 text-right">Weight</th>
                <th className="p-4 text-right">Cost</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-stone-800/50">
              {results?.map(({ item, ...r }) => (
                <tr
                  key={item.id}
                  onClick={() => setSelected(item.id)}
                  className="hover:bg-stone-800/40 cursor-pointer transition-colors"
                >
                  <td className="p-4 font-medium text-stone-200 flex items-center gap-3">
                    <ItemThumb imageUrl={item.image_url} category={item.category} />
                    {item.name}
                    {item.origin === 'import' && !item.verified && (
                      <Badge
                        className="bg-amber-950/60 text-amber-500 border border-amber-900/50"
                        title="Imported from a store and not yet vouched for"
                      >
                        unverified
                      </Badge>
                    )}
                  </td>
                  <td className="p-4">
                    <Badge className="bg-stone-800 text-stone-400">{item.category}</Badge>
                  </td>
                  <td className="p-4">
                    <ResultTags result={{ item, ...r }} scope={scope} onTag={addTag} max={3} />
                  </td>
                  <td className="p-4 text-right font-mono text-stone-400">
                    {formatGrams(Number(item.base_metadata?.[CORE]?.weight_g ?? 0))}
                  </td>
                  <td className="p-4 text-right font-mono text-stone-400">
                    {formatCost(Number(item.base_metadata?.[CORE]?.cost_cents ?? 0))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {selected && (
        <ItemInspector itemId={selected} communitySlug={communitySlug} onClose={() => setSelected(null)} />
      )}

      {importing && (
        <ImportItemModal
          onClose={() => setImporting(false)}
          onImported={(item) => {
            search.reload();
            // Drop straight into the inspector so the user can add their own layer
            // (personal weight measurement, notes) while the item is fresh in mind.
            setSelected(item.id);
          }}
        />
      )}
    </div>
  );
}

/**
 * The item inspector is where the layering model becomes visible: every resolved value is
 * tagged with the layer that produced it, and you can edit your own public/private layers.
 */
function ItemInspector({
  itemId,
  communitySlug,
  onClose,
}: {
  itemId: string;
  communitySlug: string;
  onClose: () => void;
}) {
  const resolved = useAsync<ResolvedItem>(() => api.getItem(itemId, { community: communitySlug }), [itemId, communitySlug]);
  const layer = useAsync<ProfileItemLayer>(() => api.getProfileLayer(itemId), [itemId]);

  const [publicJson, setPublicJson] = useState<string | null>(null);
  const [privateJson, setPrivateJson] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  const publicValue = publicJson ?? JSON.stringify(layer.data?.public_metadata ?? {}, null, 2);
  const privateValue = privateJson ?? JSON.stringify(layer.data?.private_metadata ?? {}, null, 2);

  async function save() {
    setStatus(null);
    try {
      await api.setProfileLayer(itemId, {
        public_metadata: JSON.parse(publicValue),
        private_metadata: JSON.parse(privateValue),
      });
      setStatus('Saved your layer.');
      resolved.reload();
      layer.reload();
    } catch (err) {
      setStatus(err instanceof SyntaxError ? 'Invalid JSON.' : (err as Error).message);
    }
  }

  return (
    <aside className="w-[28rem] border-l border-stone-900 bg-stone-950 overflow-y-auto shrink-0">
      <div className="p-5 border-b border-stone-900 flex gap-4 items-start">
        {resolved.data && (
          <ItemThumb
            imageUrl={resolved.data.image_url}
            category={resolved.data.category}
            size="w-16 h-16"
            iconSize="w-7 h-7"
          />
        )}
        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-medium text-white">{resolved.data?.name ?? 'Item'}</h2>
          <div className="text-[11px] text-stone-500 mt-1">{itemId}</div>
        </div>
        <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="p-5 space-y-6">
        {resolved.loading && <Spinner />}
        {resolved.error && <ErrorNote message={resolved.error} />}

        {resolved.data && (
          <>
            <div>
              <div className="text-[10px] uppercase font-bold tracking-wider text-stone-500 mb-2">Applied layers</div>
              <div className="flex flex-wrap gap-1.5">
                {resolved.data.applied_layers.map((name) => (
                  <Badge key={name} className={LAYER_STYLES[name].className}>
                    {LAYER_STYLES[name].label}
                  </Badge>
                ))}
              </div>
              {!communitySlug && (
                <p className="text-[11px] text-stone-600 mt-2">
                  Pick a community lens above to see community-specific metadata.
                </p>
              )}
            </div>

            <div>
              <div className="text-[10px] uppercase font-bold tracking-wider text-stone-500 mb-2">
                Resolved metadata
              </div>
              <div className="space-y-3">
                {Object.entries(resolved.data.metadata).map(([namespace, values]) => (
                  <div key={namespace} className="rounded-lg border border-stone-800 overflow-hidden">
                    <div className="px-3 py-1.5 bg-stone-900 text-[11px] font-mono text-stone-400">{namespace}</div>
                    <div className="divide-y divide-stone-900">
                      {Object.entries(values as Record<string, unknown>).map(([key, value]) => {
                        const origin = resolved.data!.provenance[`${namespace}.${key}`] ?? 'global';
                        return (
                          <div key={key} className="px-3 py-2 flex items-center justify-between gap-2 text-xs">
                            <span className="text-stone-500 font-mono">{key}</span>
                            <span className="flex items-center gap-2">
                              <span className="text-stone-200 font-mono">{String(value)}</span>
                              <Badge className={LAYER_STYLES[origin].className}>{LAYER_STYLES[origin].label}</Badge>
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <div className="text-[10px] uppercase font-bold tracking-wider text-emerald-400 mb-1">
                Your public layer
              </div>
              <p className="text-[11px] text-stone-600 mb-2">
                Overrides and additions others can see on your loadouts.
              </p>
              <textarea
                value={publicValue}
                onChange={(e) => setPublicJson(e.target.value)}
                rows={5}
                spellCheck={false}
                className="w-full bg-black/40 border border-stone-800 rounded-lg p-3 text-xs font-mono text-emerald-200 outline-none focus:border-emerald-500"
              />
            </div>

            <div>
              <div className="text-[10px] uppercase font-bold tracking-wider text-purple-400 mb-1">
                Your private layer
              </div>
              <p className="text-[11px] text-stone-600 mb-2">Notes only you can read. Never served to other profiles.</p>
              <textarea
                value={privateValue}
                onChange={(e) => setPrivateJson(e.target.value)}
                rows={5}
                spellCheck={false}
                className="w-full bg-black/40 border border-stone-800 rounded-lg p-3 text-xs font-mono text-purple-200 outline-none focus:border-purple-500"
              />
            </div>

            <div className="flex items-center gap-3">
              <button
                onClick={save}
                className="px-4 py-2 rounded-lg bg-stone-800 hover:bg-stone-700 text-sm font-medium text-stone-200"
              >
                Save my layer
              </button>
              {status && <span className="text-xs text-stone-400">{status}</span>}
            </div>

            {/* Item-scoped plugins render last: they annotate the item rather than define it. */}
            <PluginSurfaceHost surface="item.tab" itemId={itemId} communityId={communitySlug || undefined} />
          </>
        )}
      </div>
    </aside>
  );
}
