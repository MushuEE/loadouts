import { Filter, X } from 'lucide-react';
import type { LoadoutFilter, TagFilter } from '../api/types';

/**
 * Turn tags on to narrow a loadout to one variant of the kit: #warmwear shows the t-shirt
 * and hides the down jacket, in the slots, the stats and the plugins alike.
 *
 * Chips are a union: turning on #warmwear and #rain shows gear tagged with either. Gear
 * with no tags at all stays visible unless you say otherwise, because the tent and the pack
 * are in every variant and nobody should have to tag them to keep seeing them.
 */
export function TagFilterBar({
  summary,
  value,
  onChange,
  canTag,
}: {
  summary: LoadoutFilter;
  value: TagFilter;
  onChange: (next: TagFilter) => void;
  /** The owner can tag gear, so an empty bar points them at how. */
  canTag: boolean;
}) {
  const on = new Set(value.tags);
  const active = value.tags.length > 0;
  const available = summary.available_tags ?? [];

  const toggle = (tag: string) =>
    onChange({ ...value, tags: on.has(tag) ? value.tags.filter((t) => t !== tag) : [...value.tags, tag] });

  if (available.length === 0 && !active) {
    if (!canTag) return null;
    return (
      <div className="flex items-center gap-2 px-8 py-2 border-b border-stone-800 text-[11px] text-stone-600">
        <Filter className="w-3.5 h-3.5 shrink-0" />
        Tag your gear (open an item, then add a tag like #warmwear) to filter this loadout down to one variant.
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 px-8 py-2 border-b border-stone-800 bg-stone-900/30 flex-wrap">
      <Filter className={`w-3.5 h-3.5 shrink-0 ${active ? 'text-orange-400' : 'text-stone-600'}`} />
      {available.map(({ tag, count }) => {
        const selected = on.has(tag);
        return (
          <button
            key={tag}
            onClick={() => toggle(tag)}
            aria-pressed={selected}
            className={`px-2 py-0.5 rounded-full text-[11px] border transition-colors ${
              selected
                ? 'bg-orange-500/15 border-orange-500/50 text-orange-200'
                : 'border-stone-800 text-stone-400 hover:border-stone-600 hover:text-stone-200'
            }`}
          >
            #{tag}
            <span className={`ml-1 tabular-nums ${selected ? 'text-orange-300/70' : 'text-stone-600'}`}>{count}</span>
          </button>
        );
      })}

      {active && (
        <>
          <label
            className="ml-2 flex items-center gap-1.5 text-[11px] text-stone-500 cursor-pointer select-none"
            title="Untagged gear usually belongs to every variant, so it is shown by default"
          >
            <input
              type="checkbox"
              checked={!value.exclude_untagged}
              onChange={(e) => onChange({ ...value, exclude_untagged: !e.target.checked })}
              className="accent-orange-500"
            />
            Include untagged
          </label>
          <span className="ml-auto text-[11px] text-stone-500 tabular-nums">
            Showing {summary.shown_entries} of {summary.total_entries}
          </span>
          <button
            onClick={() => onChange({ tags: [], exclude_untagged: false })}
            className="flex items-center gap-1 text-[11px] text-stone-500 hover:text-white"
          >
            <X className="w-3 h-3" /> Clear
          </button>
        </>
      )}
    </div>
  );
}
