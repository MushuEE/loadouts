import { useMemo, useState } from 'react';
import { Search, X } from 'lucide-react';
import { api } from '../api/client';
import type { Item, SlotDefinition } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { CORE, categoryIcon, formatCost, formatGrams } from '../lib/display';
import { ErrorNote, Spinner } from './ui';

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
  const items = useAsync<Item[]>(() => api.listItems(), []);

  const accepted = slot.accepted_categories ?? [];
  const universal = accepted.length === 0 || accepted.includes('universal');

  const visible = useMemo(() => {
    const list = items.data ?? [];
    return list
      .filter((item) => universal || accepted.includes(item.category))
      .filter((item) => item.name.toLowerCase().includes(query.toLowerCase()));
  }, [items.data, accepted, universal, query]);

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
            <div className="py-12 text-center text-stone-600 text-sm">
              No compatible gear. Add it in the Garage first.
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
      </div>
    </div>
  );
}
