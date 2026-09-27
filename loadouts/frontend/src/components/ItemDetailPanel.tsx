import { useState } from 'react';
import { X } from 'lucide-react';
import { api } from '../api/client';
import { useAsync } from '../lib/useAsync';
import { useSession } from '../session/SessionContext';
import type { LayerName, ResolvedEntry } from '../api/types';
import { CORE, LAYER_STYLES, categoryIcon, formatCost, formatGrams, itemTags } from '../lib/display';

/**
 * Everything known about one item in a loadout, including where each fact came from.
 *
 * The provenance display is the point. An item's metadata is assembled from four layers -
 * global, community, user_public, user_private - and by the time it reaches the UI it is a
 * single flat object with the seams hidden. Showing which layer won for each key is what
 * turns "the weight is 812g" into "the weight is 812g *because you measured it*, overriding
 * the manufacturer's 790g".
 */


export function ItemDetailPanel({
  node,
  onClose,
  ownerHandle,
  onTagsChanged,
}: {
  node: ResolvedEntry;
  onClose: () => void;
  /** Set when the item is shown in someone else's loadout, whose tags it carries. */
  ownerHandle?: string | null;
  /** Called after the viewer changes their own tags. */
  onTagsChanged?: () => void;
}) {
  const item = node.item;
  const core = (item.metadata?.[CORE] ?? {}) as Record<string, unknown>;
  const weight = Number(core.weight_g ?? 0);
  const cost = Number(core.cost_cents ?? 0);
  const description = String(core.description ?? '');
  const consumable = Boolean(core.consumable);

  // Namespaces other than core are community or plugin additions - ul_score, drip_score
  // and so on. They are the interesting part of the layer model, so they get equal billing
  // rather than being buried.
  const extraNamespaces = Object.entries(item.metadata ?? {}).filter(
    ([ns, values]) => ns !== CORE && values && Object.keys(values).length > 0,
  );

  const layerOf = (ns: string, key: string): LayerName | undefined =>
    item.provenance?.[`${ns}.${key}`];

  return (
    <aside className="w-80 shrink-0 border-l border-stone-800 bg-stone-900 overflow-y-auto">
      <div className="sticky top-0 bg-stone-900 border-b border-stone-800 px-4 py-3 flex items-start gap-3">
        <span className="text-xl leading-none mt-0.5">{categoryIcon(item.category)}</span>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium text-stone-100 leading-tight">{item.name}</div>
          <div className="text-[11px] text-stone-500 mt-0.5">{item.category}</div>
        </div>
        <button onClick={onClose} className="text-stone-600 hover:text-white shrink-0" title="Close">
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="p-4 space-y-5">
        {item.image_url && (
          <img
            src={item.image_url}
            alt=""
            referrerPolicy="no-referrer"
            onError={(e) => ((e.target as HTMLImageElement).style.display = 'none')}
            className="w-full rounded-lg bg-stone-100 object-contain max-h-40"
          />
        )}

        {description && <p className="text-xs text-stone-400 leading-relaxed">{description}</p>}

        <div className="grid grid-cols-2 gap-2">
          <Stat label="Weight" value={formatGrams(weight)} layer={layerOf(CORE, 'weight_g')} />
          <Stat label="Cost" value={formatCost(cost)} layer={layerOf(CORE, 'cost_cents')} />
          <Stat label="Quantity" value={String(node.entry.quantity || 1)} />
          <Stat label="Counts as" value={consumable ? 'Consumable' : 'Base weight'} layer={layerOf(CORE, 'consumable')} />
        </div>

        <TagEditor
          key={item.id}
          itemId={item.id}
          ownerHandle={ownerHandle ?? null}
          ownerTags={itemTags(item)}
          onChanged={onTagsChanged}
        />

        {node.entry.note && (
          <Section title="Your note">
            <p className="text-xs text-stone-400 italic leading-relaxed">{node.entry.note}</p>
          </Section>
        )}

        {extraNamespaces.map(([ns, values]) => (
          <Section key={ns} title={ns.replace(/_/g, ' ')}>
            <div className="space-y-1">
              {Object.entries(values as Record<string, unknown>).map(([key, value]) => (
                <div key={key} className="flex items-center justify-between gap-2 text-xs">
                  <span className="text-stone-500 truncate">{key.replace(/_/g, ' ')}</span>
                  <span className="flex items-center gap-1.5 shrink-0">
                    <span className="text-stone-300 tabular-nums">{String(value)}</span>
                    <LayerBadge layer={layerOf(ns, key)} />
                  </span>
                </div>
              ))}
            </div>
          </Section>
        ))}

        {item.applied_layers?.length > 0 && (
          <Section title="Layers applied">
            <div className="flex flex-wrap gap-1">
              {item.applied_layers.map((l) => (
                <LayerBadge key={l} layer={l} />
              ))}
            </div>
            <p className="text-[10px] text-stone-600 mt-2 leading-relaxed">
              Later layers override earlier ones. A private value never leaves your account.
            </p>
          </Section>
        )}

        {item.sources && item.sources.length > 0 && (
          <Section title="Where to buy">
            <div className="space-y-1">
              {item.sources.map((s, i) => (
                <a
                  key={i}
                  href={s.url}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="flex items-center justify-between gap-2 text-xs text-stone-400 hover:text-orange-300"
                >
                  <span className="truncate">{s.supplier_name}</span>
                  <span className="tabular-nums shrink-0">{formatCost(s.price)}</span>
                </a>
              ))}
            </div>
          </Section>
        )}

        {item.origin && (
          <Section title="Origin">
            <div className="text-xs text-stone-400">
              {item.origin === 'import' ? 'Imported from a store page' : 'Curated catalogue entry'}
              {!item.verified && (
                <span className="ml-2 text-[10px] text-amber-400/80">unverified</span>
              )}
            </div>
          </Section>
        )}
      </div>
    </aside>
  );
}

/**
 * Tags are per-profile: each person labels their own gear, and nobody's labels overwrite
 * anyone else's. This shows the three views of that for one item:
 *
 *  - yours, editable, which are what your own loadouts filter by;
 *  - the loadout owner's, read-only, when you are looking at someone else's kit;
 *  - everyone's, as counts, which double as suggestions you can adopt with a click.
 */
function TagEditor({
  itemId,
  ownerHandle,
  ownerTags,
  onChanged,
}: {
  itemId: string;
  ownerHandle: string | null;
  ownerTags: string[];
  onChanged?: () => void;
}) {
  const { profile } = useSession();
  const [nonce, setNonce] = useState(0);
  const tags = useAsync(() => api.getItemTags(itemId), [itemId, profile?.id, nonce]);
  const popular = useAsync(() => api.popularTags('', 50), [profile?.id, nonce]);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mine = tags.data?.mine ?? [];
  const global = tags.data?.global ?? [];
  const listId = `tag-suggestions-${itemId}`;

  async function save(next: string[]) {
    setBusy(true);
    setError(null);
    try {
      await api.setItemTags(itemId, next);
      setDraft('');
      setNonce((n) => n + 1);
      onChanged?.();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Section title="Tags">
      {ownerHandle && ownerTags.length > 0 && (
        <TagRow label={`@${ownerHandle}`}>
          {ownerTags.map((t) => (
            <Chip key={t}>#{t}</Chip>
          ))}
        </TagRow>
      )}

      {profile && (
        <TagRow label={ownerHandle ? 'Yours' : undefined}>
          {mine.map((t) => (
            <Chip key={t}>
              #{t}
              <button
                onClick={() => save(mine.filter((x) => x !== t))}
                disabled={busy}
                className="text-stone-500 hover:text-red-400"
                title={`Remove #${t}`}
              >
                <X className="w-3 h-3" />
              </button>
            </Chip>
          ))}
          <form
            onSubmit={(e) => {
              e.preventDefault();
              // Commas separate tags; spaces inside one become hyphens on the server, so
              // "summer trips" is the single tag #summer-trips.
              const added = draft.split(',').map((t) => t.trim()).filter(Boolean);
              if (added.length) save([...mine, ...added]);
            }}
          >
            <input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
              list={listId}
              placeholder={mine.length ? 'Add tag' : '#warmwear'}
              className="w-24 px-2 py-0.5 rounded-full bg-stone-950 border border-stone-800 text-[11px] text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-orange-500/60"
            />
            <datalist id={listId}>
              {(popular.data?.tags ?? [])
                .filter((t) => !mine.includes(t.tag))
                .map((t) => (
                  <option key={t.tag} value={t.tag} />
                ))}
            </datalist>
          </form>
        </TagRow>
      )}

      {global.length > 0 && (
        <TagRow label="Everyone">
          {global.map(({ tag, count }) => {
            const have = mine.includes(tag);
            return (
              <button
                key={tag}
                onClick={() => !have && profile && save([...mine, tag])}
                disabled={busy || have || !profile}
                title={
                  have
                    ? `You tag this #${tag} too`
                    : `${count} ${count === 1 ? 'person tags' : 'people tag'} this #${tag}. Click to add it to yours.`
                }
                className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-[11px] ${
                  have
                    ? 'border-orange-500/30 text-orange-200/80'
                    : 'border-stone-800 text-stone-400 hover:border-stone-600 hover:text-stone-200'
                }`}
              >
                #{tag}
                <span className="tabular-nums text-stone-600">{count}</span>
              </button>
            );
          })}
        </TagRow>
      )}

      {error && <p className="text-[10px] text-red-300 mt-1.5">{error}</p>}
      <p className="text-[10px] text-stone-600 mt-2 leading-relaxed">
        Your tags are yours alone and follow this item into every loadout of yours, where the filter bar can
        switch between them.
      </p>
    </Section>
  );
}

function TagRow({ label, children }: { label?: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2 mb-1.5">
      {label && <span className="w-14 shrink-0 pt-1 text-[10px] text-stone-500 truncate">{label}</span>}
      <div className="flex flex-wrap gap-1.5 min-w-0">{children}</div>
    </div>
  );
}

function Chip({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-stone-800 text-[11px] text-stone-300">
      {children}
    </span>
  );
}

function Stat({ label, value, layer }: { label: string; value: string; layer?: LayerName }) {
  return (
    <div className="p-2.5 rounded-lg bg-stone-800/60 border border-stone-800">
      <div className="text-[9px] font-bold uppercase tracking-widest text-stone-500 mb-1 flex items-center gap-1">
        {label}
        {/* Only annotate a value that did not come from the shared catalogue. Badging
            everything would make the interesting overrides invisible in the noise. */}
        {layer && layer !== 'global' && <LayerBadge layer={layer} />}
      </div>
      <div className="text-sm text-stone-200 tabular-nums">{value}</div>
    </div>
  );
}

function LayerBadge({ layer }: { layer?: LayerName }) {
  if (!layer) return null;
  const style = LAYER_STYLES[layer];
  if (!style) return null;
  return (
    <span
      title={style.hint}
      className={`px-1.5 py-px rounded text-[8px] font-medium uppercase tracking-wider whitespace-nowrap ${style.className}`}
    >
      {style.label}
    </span>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[9px] font-bold uppercase tracking-widest text-stone-500 mb-2">{title}</div>
      {children}
    </div>
  );
}
