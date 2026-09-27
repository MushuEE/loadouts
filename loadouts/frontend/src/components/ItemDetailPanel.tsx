import { X } from 'lucide-react';
import type { LayerName, ResolvedEntry } from '../api/types';
import { CORE, LAYER_STYLES, categoryIcon, formatCost, formatGrams } from '../lib/display';

/**
 * Everything known about one item in a loadout, including where each fact came from.
 *
 * The provenance display is the point. An item's metadata is assembled from four layers -
 * global, community, user_public, user_private - and by the time it reaches the UI it is a
 * single flat object with the seams hidden. Showing which layer won for each key is what
 * turns "the weight is 812g" into "the weight is 812g *because you measured it*, overriding
 * the manufacturer's 790g".
 */


export function ItemDetailPanel({ node, onClose }: { node: ResolvedEntry; onClose: () => void }) {
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
          <img src={item.image_url} alt="" className="w-full rounded-lg bg-stone-950 object-contain max-h-40" />
        )}

        {description && <p className="text-xs text-stone-400 leading-relaxed">{description}</p>}

        <div className="grid grid-cols-2 gap-2">
          <Stat label="Weight" value={formatGrams(weight)} layer={layerOf(CORE, 'weight_g')} />
          <Stat label="Cost" value={formatCost(cost)} layer={layerOf(CORE, 'cost_cents')} />
          <Stat label="Quantity" value={String(node.entry.quantity || 1)} />
          <Stat label="Counts as" value={consumable ? 'Consumable' : 'Base weight'} layer={layerOf(CORE, 'consumable')} />
        </div>

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
