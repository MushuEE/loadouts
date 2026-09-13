import { useMemo, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import type { ResolvedEntry, SlotDefinition } from '../api/types';
import { categoryIcon, formatGrams } from '../lib/display';
import {
  columnForTarget,
  isMapped,
  targetForSlot,
  type BodyPart,
  type PropName,
} from '../paperdoll/archetypes';
import { TentProp, silhouetteFor } from '../paperdoll/silhouettes';

/**
 * Equipment laid out around a figure, WoW-style: slot frames flank the silhouette rather
 * than listing beside it.
 *
 * Hovering a populated slot lights the corresponding part of the figure. Hover rather than
 * selection is the point - it is exploratory, so you can sweep the slots and watch the
 * figure respond without clicking anything or leaving state behind to undo.
 *
 * Empty slots still render. A slot is a statement about what the template expects, so an
 * unfilled one is information: it is the gap in your kit. Hiding them would conceal exactly
 * what a gear list exists to show.
 */
export function Paperdoll({
  templateName,
  slots,
  entries,
  readOnly,
  onPick,
  onRemove,
}: {
  templateName: string;
  slots: SlotDefinition[];
  entries: ResolvedEntry[];
  readOnly: boolean;
  onPick: (slot: SlotDefinition) => void;
  onRemove: (node: ResolvedEntry) => void;
}) {
  const [hovered, setHovered] = useState<string | null>(null);

  const mapped = useMemo(
    () => slots.map((slot) => ({ slot, target: targetForSlot(slot) })).filter((s) => isMapped(s.target)),
    [slots],
  );

  const occupantsOf = (slotId: string) => entries.filter((e) => e.entry.slot_id === slotId);

  // Only a populated slot lights the figure. A lit limb with nothing in it would be
  // pointing at gear that is not there.
  const active = useMemo(() => {
    const set = new Set<BodyPart | PropName>();
    if (!hovered) return set;
    const hit = mapped.find((m) => m.slot.id === hovered);
    if (!hit || occupantsOf(hovered).length === 0) return set;
    if (hit.target.kind === 'body') set.add(hit.target.part);
    if (hit.target.kind === 'prop') set.add(hit.target.prop);
    return set;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hovered, mapped, entries]);

  if (mapped.length === 0) return null;

  const Silhouette = silhouetteFor(templateName);
  const left = mapped.filter((m) => columnForTarget(m.target) === 'left');
  const right = mapped.filter((m) => columnForTarget(m.target) === 'right');
  const showTent = mapped.some((m) => m.target.kind === 'prop' && m.target.prop === 'tent');

  const column = (items: typeof mapped) => (
    <div className="flex flex-col gap-2.5">
      {items.map(({ slot }) => (
        <PaperdollSlot
          key={slot.id}
          slot={slot}
          occupants={occupantsOf(slot.id)}
          readOnly={readOnly}
          hovered={hovered === slot.id}
          onHover={setHovered}
          onPick={() => onPick(slot)}
          onRemove={onRemove}
        />
      ))}
    </div>
  );

  return (
    <div className="mb-8 p-5 rounded-xl bg-stone-900/60 border border-stone-800">
      <div className="flex items-center justify-between mb-4">
        <span className="text-[10px] font-bold uppercase tracking-widest text-stone-500">Equipped</span>
        <span className="text-[10px] text-stone-600">Hover a slot to locate it</span>
      </div>

      <div className="grid grid-cols-[1fr_auto_1fr] gap-4 items-center">
        {column(left)}
        <div className="w-[150px] sm:w-[190px] flex flex-col items-center gap-2">
          <Silhouette active={active} />
          {showTent && (
            <div className="w-16 opacity-80">
              <TentProp active={active} />
            </div>
          )}
        </div>
        {column(right)}
      </div>
    </div>
  );
}

function PaperdollSlot({
  slot,
  occupants,
  readOnly,
  hovered,
  onHover,
  onPick,
  onRemove,
}: {
  slot: SlotDefinition;
  occupants: ResolvedEntry[];
  readOnly: boolean;
  hovered: boolean;
  onHover: (slotId: string | null) => void;
  onPick: () => void;
  onRemove: (node: ResolvedEntry) => void;
}) {
  const filled = occupants.length > 0;
  // max_items 0 means "exactly one", -1 means unlimited. See core.SlotDefinition.
  const capacity = slot.max_items === -1 ? Infinity : Math.max(slot.max_items, 1);
  const full = occupants.length >= capacity;

  return (
    <div
      onMouseEnter={() => onHover(slot.id)}
      onMouseLeave={() => onHover(null)}
      className={`rounded-lg border p-2.5 transition-colors ${
        hovered && filled
          ? 'border-amber-400/70 bg-amber-400/5'
          : filled
            ? 'border-stone-700 bg-stone-900'
            : 'border-dashed border-stone-800 bg-stone-900/40'
      }`}
    >
      <div className="flex items-center justify-between gap-2 mb-1.5">
        <span className="text-[9px] font-bold uppercase tracking-widest text-stone-500 truncate">
          {slot.name}
          {slot.required && <span className="text-orange-500/80"> *</span>}
        </span>
        {!readOnly && !full && (
          <button onClick={onPick} className="text-stone-600 hover:text-orange-400 shrink-0" title="Add item">
            <Plus className="w-3 h-3" />
          </button>
        )}
      </div>

      {filled ? (
        <div className="space-y-1">
          {occupants.map((node) => (
            <div key={node.entry.id} className="group flex items-center gap-1.5">
              <span className="text-xs shrink-0">{categoryIcon(node.item.category)}</span>
              <span className="text-[11px] text-stone-300 truncate flex-1">{node.item.name}</span>
              <span className="text-[10px] text-stone-600 tabular-nums shrink-0">
                {formatGrams(Number(node.item.metadata?.core?.weight_g ?? 0))}
              </span>
              {!readOnly && (
                <button
                  onClick={() => onRemove(node)}
                  className="opacity-0 group-hover:opacity-100 text-stone-600 hover:text-red-400 shrink-0"
                  title="Remove"
                >
                  <Trash2 className="w-3 h-3" />
                </button>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="text-[11px] text-stone-600 italic px-0.5">Empty</div>
      )}
    </div>
  );
}
