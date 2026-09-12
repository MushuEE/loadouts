import { useMemo, useState } from 'react';
import { PackagePlus, Search, X } from 'lucide-react';
import { api } from '../api/client';
import type { Item, SlotDefinition } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { CORE, categoryIcon, formatCost, formatGrams } from '../lib/display';
import { ErrorNote, Spinner } from './ui';
import { ImportItemModal } from './ImportItemModal';

/**
 * Item picker constrained by the slot's accepted categories, so the template's structure
 * guides the user instead of only failing validation after the fact.
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
  const items = useAsync<Item[]>(() => api.listItems(), []);

  const accepted = slot.accepted_categories ?? [];
  const universal = accepted.length === 0 || accepted.includes('universal');

  const visible = useMemo(() => {
    const list = items.data ?? [];
    return list
      .filter((item) => universal || accepted.includes(item.category))
      .filter((item) => item.name.toLowerCase().includes(query.toLowerCase()));
  }, [items.data, accepted, universal, query]);

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
            <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="flex items-center bg-black/40 border border-stone-800 rounded-lg px-3 mt-3 focus-within:border-orange-500">
            <Search className="w-4 h-4 text-stone-500" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter…"
              className="bg-transparent px-3 py-2 text-sm text-white outline-none flex-1 placeholder:text-stone-600"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {items.loading && <Spinner />}
          {items.error && <div className="p-2"><ErrorNote message={items.error} /></div>}
          {!items.loading && visible.length === 0 && (
            <div className="py-12 text-center">
              <p className="text-stone-600 text-sm">No compatible gear in the catalog.</p>
              <button
                onClick={() => setImporting(true)}
                className="mt-3 px-4 py-2 bg-orange-600 hover:bg-orange-500 text-white text-sm font-medium rounded-lg inline-flex items-center gap-2"
              >
                <PackagePlus className="w-4 h-4" />
                Import from a store
              </button>
            </div>
          )}
          {visible.map((item) => (
            <button
              key={item.id}
              onClick={() => onSelect(item.id)}
              className="w-full text-left p-3 rounded-lg hover:bg-stone-800 border border-transparent hover:border-stone-700 flex items-center justify-between group"
            >
              <span className="flex items-center gap-3">
                <span className="p-2 bg-stone-800 rounded-md text-stone-400">{categoryIcon(item.category, 'w-4 h-4')}</span>
                <span>
                  <span className="block text-stone-200 text-sm group-hover:text-orange-400">{item.name}</span>
                  <span className="block text-[10px] uppercase tracking-wider text-stone-600">{item.category}</span>
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
