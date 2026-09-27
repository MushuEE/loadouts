import { useMemo, useState } from 'react';
import { PackagePlus, Search, X } from 'lucide-react';
import { api } from '../api/client';
import type { GearResult, GearSearch, SlotDefinition } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { useGearScope } from '../lib/gearScope';
import { ScopeToggle } from './ScopeToggle';
import { ResultTags, normalizeTag } from './GearSearchControls';
import { CORE, formatCost, formatGrams } from '../lib/display';
import { ItemThumb } from './ItemThumb';
import { ErrorNote, Spinner } from './ui';
import { ImportItemModal } from './ImportItemModal';

/**
 * Item picker constrained by the slot's accepted categories, so the template's structure
 * guides the user instead of only failing validation after the fact.
 *
 * It follows the app-wide scope: "My gear" first is usually what you want when packing,
 * with Everyone one click away. Typing #tag narrows by tag, in the scope's sense.
 */
export function ItemPickerModal({
  slot,
  onClose,
  onSelect,
}: {
  slot: SlotDefinition;
  onClose: () => void;
  onSelect: (itemId: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [importing, setImporting] = useState(false);
  const [scope, setScope] = useGearScope();
  const items = useAsync<GearSearch>(() => api.searchGear({ scope }), [scope]);

  const accepted = slot.accepted_categories ?? [];
  const universal = accepted.length === 0 || accepted.includes('universal');

  const visible = useMemo(() => {
    const words = query.split(/\s+/).filter(Boolean);
    const tags = words.filter((w) => w.startsWith('#')).map(normalizeTag).filter(Boolean);
    const text = words.filter((w) => !w.startsWith('#')).join(' ').toLowerCase();
    const tagsOf = (r: GearResult) => (scope === 'mine' ? r.my_tags : r.tags.map((t) => t.tag));
    return (items.data?.results ?? [])
      .filter(({ item }) => universal || accepted.includes(item.category))
      .filter(({ item }) => item.name.toLowerCase().includes(text))
      .filter((r) => tags.every((t) => tagsOf(r).some((have) => have.startsWith(t))));
  }, [items.data, accepted, universal, query, scope]);

  // Importing mid-build is the common case: you're filling a slot and realize the piece
  // of gear isn't in the catalog yet. Sending the user off to the Garage would lose the
  // slot context, so the picker hands the imported item straight back to the slot.
  if (importing) {
    return (
      <ImportItemModal
        onClose={() => setImporting(false)}
        onImported={(item) => onSelect(item.id)}
      />
    );
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-stone-900 border border-stone-700 rounded-xl w-full max-w-lg max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-stone-800">
          <div className="flex justify-between items-center">
            <div>
              <h3 className="font-bold text-white">Select gear</h3>
              <p className="text-[11px] text-stone-500 mt-0.5">
                Slot: {slot.name} · accepts {universal ? 'anything' : accepted.join(', ')}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <ScopeToggle size="sm" />
              <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>
          <div className="flex items-center bg-black/40 border border-stone-800 rounded-lg px-3 mt-3 focus-within:border-orange-500">
            <Search className="w-4 h-4 text-stone-500" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter, or #tag…"
              className="bg-transparent px-3 py-2 text-sm text-white outline-none flex-1 placeholder:text-stone-600"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {items.loading && !items.data && <Spinner />}
          {items.error && <div className="p-2"><ErrorNote message={items.error} /></div>}
          {!items.loading && visible.length === 0 && (
            <div className="py-12 text-center">
              <p className="text-stone-600 text-sm">
                {scope === 'mine' ? 'None of your gear fits here.' : 'No compatible gear in the catalog.'}
              </p>
              {scope === 'mine' && (
                <button onClick={() => setScope('everyone')} className="mt-2 block mx-auto text-xs text-orange-400 hover:text-orange-300">
                  Search everyone's gear
                </button>
              )}
              <button
                onClick={() => setImporting(true)}
                className="mt-3 px-4 py-2 bg-orange-600 hover:bg-orange-500 text-white text-sm font-medium rounded-lg inline-flex items-center gap-2"
              >
                <PackagePlus className="w-4 h-4" />
                Import from a store
              </button>
            </div>
          )}
          {visible.map(({ item, ...r }) => (
            <button
              key={item.id}
              onClick={() => onSelect(item.id)}
              className="w-full text-left p-3 rounded-lg hover:bg-stone-800 border border-transparent hover:border-stone-700 flex items-center justify-between group"
            >
              <span className="flex items-center gap-3">
                <ItemThumb imageUrl={item.image_url} category={item.category} size="w-10 h-10" />
                <span>
                  <span className="block text-stone-200 text-sm group-hover:text-orange-400">{item.name}</span>
                  <span className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-stone-600">
                    {item.category}
                    {r.mine && scope === 'everyone' && <span className="normal-case tracking-normal text-orange-400">· yours</span>}
                  </span>
                  <span className="block mt-1">
                    <ResultTags result={{ item, ...r }} scope={scope} max={3} />
                  </span>
                </span>
              </span>
              <span className="text-right font-mono text-xs text-stone-400">
                <span className="block">{formatGrams(Number(item.base_metadata?.[CORE]?.weight_g ?? 0))}</span>
                <span className="block text-stone-600">{formatCost(Number(item.base_metadata?.[CORE]?.cost_cents ?? 0))}</span>
              </span>
            </button>
          ))}
        </div>

        <div className="p-3 border-t border-stone-800">
          <button
            onClick={() => setImporting(true)}
            className="w-full py-2 text-xs text-stone-400 hover:text-orange-400 flex items-center justify-center gap-2"
          >
            <PackagePlus className="w-3.5 h-3.5" />
            Can't find it? Import from a store link
          </button>
        </div>
      </div>
    </div>
  );
}
