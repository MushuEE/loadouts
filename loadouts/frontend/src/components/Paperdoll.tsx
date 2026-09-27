import { useMemo, useState, type ReactNode } from 'react';
import { EyeOff, Maximize2, Plus, Trash2 } from 'lucide-react';
import type { ResolvedEntry, SlotDefinition } from '../api/types';
import { formatGrams } from '../lib/display';
import { ItemThumb } from './ItemThumb';
import {
  columnForTarget,
  isMapped,
  targetForSlot,
  type BodyPart,
  type PropName,
} from '../paperdoll/archetypes';
import { TentProp, silhouetteFor } from '../paperdoll/silhouettes';

/**
 * Everything a slot frame needs to act on the loadout. The editor builds one of these and
 * both paperdolls pass it through untouched, so a slot behaves the same wherever it lands.
 */
export interface SlotActions {
  readOnly: boolean;
  selectedEntryId: string | null;
  /** Slots the owner added to this loadout. They get a badge and can be removed. */
  customSlotIds: Set<string>;
  onPick: (slot: SlotDefinition) => void;
  onRemove: (node: ResolvedEntry) => void;
  onSelect: (entryId: string | null) => void;
  /** Open a container (a pack, a ditty bag) to see the slots inside it. */
  onZoom: (node: ResolvedEntry) => void;
  onRemoveSlot?: (slot: SlotDefinition) => void;
}

/**
 * Split slots into the two columns flanking the figure and a row underneath, WoW-style.
 *
 * `left` and `right` are the slots that point at something on the figure. Everything else
 * (unplaced template slots, slots the owner invented) tops up the shorter column first, so
 * the frame stays a frame instead of a figure with a list dumped under it, and only
 * overflows to the bottom row once the columns are as long as the figure is tall.
 */
export function wrapAround<T>(left: T[], right: T[], extras: T[], minColumn = 4) {
  const l = [...left];
  const r = [...right];
  const bottom: T[] = [];
  const cap = Math.max(l.length, r.length, minColumn);
  for (const s of extras) {
    const col = l.length <= r.length ? l : r;
    if (col.length < cap) col.push(s);
    else bottom.push(s);
  }
  return { left: l, right: r, bottom };
}

/**
 * The frame both paperdolls share: a header, the figure flanked by slot columns, and a
 * row underneath for whatever did not fit beside it.
 */
export function PaperdollFrame({
  hint,
  action,
  left,
  figure,
  right,
  bottom,
}: {
  hint?: string;
  action?: ReactNode;
  left: ReactNode[];
  figure: ReactNode;
  right: ReactNode[];
  bottom: ReactNode[];
}) {
  const flanked = left.length > 0 || right.length > 0;
  return (
    <div className="mb-8 p-5 rounded-xl bg-stone-900/60 border border-stone-800">
      <div className="flex items-center justify-between gap-3 mb-4">
        <span className="text-[10px] font-bold uppercase tracking-widest text-stone-500">Equipped</span>
        <div className="flex items-center gap-3">
          {hint && <span className="text-[10px] text-stone-600">{hint}</span>}
          {action}
        </div>
      </div>

      {flanked ? (
        // Columns stretch to the figure's height and spread their frames along it, so a
        // short column does not bunch up at the top of a tall figure.
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_minmax(0,1fr)] gap-4 items-stretch">
          <div className="flex flex-col justify-around gap-2.5 min-w-0">{left}</div>
          <div className="self-center min-w-0">{figure}</div>
          <div className="flex flex-col justify-around gap-2.5 min-w-0">{right}</div>
        </div>
      ) : (
        <div className="max-w-xl mx-auto">{figure}</div>
      )}

      {bottom.length > 0 && (
        <div className="mt-4 grid grid-cols-[repeat(auto-fill,minmax(11rem,1fr))] gap-2.5">{bottom}</div>
      )}
    </div>
  );
}

/**
 * Equipment laid out around a figure, WoW-style: slot frames flank the silhouette rather
 * than listing beside it.
 *
 * This is the fallback for templates without an authored layout, placing slots by
 * category. Slots it cannot place still live in the frame, filling out the columns and
 * then the row underneath, so nothing you add to the loadout drops out of the paperdoll.
 *
 * Hovering a populated slot lights the corresponding part of the figure; clicking an item
 * pins its details open in the inspector. The split is deliberate - hover is exploratory, so
 * you can sweep the slots and watch the figure respond without leaving state behind to undo,
 * while a click is a commitment and gets a commitment's worth of detail.
 *
 * Empty slots still render. A slot is a statement about what the template expects, so an
 * unfilled one is information: it is the gap in your kit. Hiding them would conceal exactly
 * what a gear list exists to show.
 */
export function Paperdoll({
  templateName,
  slots,
  entries,
  actions,
  action,
  footer,
}: {
  templateName: string;
  slots: SlotDefinition[];
  entries: ResolvedEntry[];
  actions: SlotActions;
  action?: ReactNode;
  /** Trailing content for the bottom row, such as the "add slot" control. */
  footer?: ReactNode;
}) {
  const [hovered, setHovered] = useState<string | null>(null);

  const placed = useMemo(() => slots.map((slot) => ({ slot, target: targetForSlot(slot) })), [slots]);
  const mapped = placed.filter((s) => isMapped(s.target));

  const visibleIn = (slotId: string) => entries.filter((e) => e.entry.slot_id === slotId && !e.hidden);

  // Only a populated slot lights the figure. A lit limb with nothing in it would be
  // pointing at gear that is not there.
  const active = new Set<BodyPart | PropName>();
  const hit = hovered ? mapped.find((m) => m.slot.id === hovered) : undefined;
  if (hit && visibleIn(hit.slot.id).length > 0) {
    if (hit.target.kind === 'body') active.add(hit.target.part);
    if (hit.target.kind === 'prop') active.add(hit.target.prop);
  }

  const Silhouette = silhouetteFor(templateName);
  const { left, right, bottom } = wrapAround(
    mapped.filter((m) => columnForTarget(m.target) === 'left').map((m) => m.slot),
    mapped.filter((m) => columnForTarget(m.target) === 'right').map((m) => m.slot),
    placed.filter((s) => !isMapped(s.target)).map((s) => s.slot),
  );
  const showTent = mapped.some((m) => m.target.kind === 'prop' && m.target.prop === 'tent');

  const frame = (slot: SlotDefinition) => (
    <PaperdollSlot
      key={slot.id}
      slot={slot}
      occupants={entries.filter((e) => e.entry.slot_id === slot.id)}
      actions={actions}
      hovered={hovered === slot.id}
      onHover={setHovered}
    />
  );

  return (
    <PaperdollFrame
      hint={mapped.length > 0 ? 'Hover a slot to locate it' : undefined}
      action={action}
      left={left.map(frame)}
      right={right.map(frame)}
      bottom={[...bottom.map(frame), ...(footer ? [<div key="footer">{footer}</div>] : [])]}
      figure={
        <div className="w-[150px] sm:w-[190px] mx-auto flex flex-col items-center gap-2">
          <Silhouette active={active} />
          {showTent && (
            <div className="w-16 opacity-80">
              <TentProp active={active} />
            </div>
          )}
        </div>
      }
    />
  );
}

/**
 * One slot's frame: its name, what is in it, and the controls for changing that.
 *
 * `occupants` is everything in the slot, including entries the tag filter hides. The
 * frame shows only the visible ones but counts all of them against capacity, since a
 * hidden jacket still takes up the slot.
 */
export function PaperdollSlot({
  slot,
  occupants,
  actions,
  hovered,
  onHover,
}: {
  slot: SlotDefinition;
  occupants: ResolvedEntry[];
  actions: SlotActions;
  hovered: boolean;
  onHover: (slotId: string | null) => void;
}) {
  const { readOnly, selectedEntryId, customSlotIds, onPick, onRemove, onSelect, onZoom, onRemoveSlot } = actions;
  const visible = occupants.filter((n) => !n.hidden);
  const hiddenCount = occupants.length - visible.length;
  const filled = visible.length > 0;
  // max_items 0 means "exactly one", -1 means unlimited. See core.SlotDefinition.
  const capacity = slot.max_items === -1 ? Infinity : Math.max(slot.max_items, 1);
  const full = occupants.length >= capacity;
  const custom = customSlotIds.has(slot.id);
  // A selected slot stays framed after the pointer leaves. Otherwise moving the mouse to the
  // inspector would drop every cue about which slot it is describing.
  const holdsSelection = visible.some((n) => n.entry.id === selectedEntryId);

  return (
    <div
      onMouseEnter={() => onHover(slot.id)}
      onMouseLeave={() => onHover(null)}
      className={`rounded-lg border p-2.5 transition-colors ${
        holdsSelection
          ? 'border-sky-400/70 bg-sky-400/5'
          : hovered && filled
            ? 'border-amber-400/70 bg-amber-400/5'
            : filled
              ? 'border-stone-700 bg-stone-900'
              : 'border-dashed border-stone-800 bg-stone-900/40'
      }`}
    >
      <div className="flex items-center gap-1.5 mb-1.5">
        <span className="text-[9px] font-bold uppercase tracking-widest text-stone-500 truncate flex-1">
          {slot.name}
          {slot.required && <span className="text-orange-500/80"> *</span>}
        </span>
        {/* Say whose slot this is. Without it a custom slot looks like part of the template,
            and its remove button looks like it would edit the template for everyone. */}
        {custom && (
          <span
            className="text-[8px] font-bold uppercase tracking-wider text-sky-400/80 shrink-0"
            title="You added this slot to this loadout. The template does not have it."
          >
            yours
          </span>
        )}
        {!readOnly && full && capacity !== Infinity && capacity > 1 && (
          <span
            className="text-[9px] text-stone-600 tabular-nums shrink-0"
            title={`This slot holds ${capacity} items. Remove one to add another.`}
          >
            {occupants.length}/{capacity}
          </span>
        )}
        {!readOnly && !full && (
          <button onClick={() => onPick(slot)} className="text-stone-600 hover:text-orange-400 shrink-0" title="Add item">
            <Plus className="w-3 h-3" />
          </button>
        )}
        {!readOnly && custom && onRemoveSlot && (
          <button
            onClick={() => onRemoveSlot(slot)}
            className="text-stone-700 hover:text-red-400 shrink-0"
            title="Remove this slot"
          >
            <Trash2 className="w-3 h-3" />
          </button>
        )}
      </div>

      {filled ? (
        <div className="space-y-1">
          {visible.map(occupantRow)}
        </div>
      ) : hiddenCount === 0 ? (
        <div className="text-[11px] text-stone-600 italic px-0.5">Empty</div>
      ) : null}

      {hiddenCount > 0 && <HiddenNote count={hiddenCount} />}
    </div>
  );

  function occupantRow(node: ResolvedEntry) {
    const selected = node.entry.id === selectedEntryId;
    const isContainer = (node.item.provided_slots ?? []).length > 0;
    return (
      <div key={node.entry.id} className="group flex items-center gap-1.5">
        <ItemThumb imageUrl={node.item.image_url} category={node.item.category} size="w-6 h-6" iconSize="w-3.5 h-3.5" />
        <button
          onClick={() => onSelect(selected ? null : node.entry.id)}
          className={`text-[11px] truncate flex-1 text-left ${selected ? 'text-sky-200' : 'text-stone-300 hover:text-white'}`}
          title="Show details"
        >
          {node.item.name}
          {node.entry.quantity > 1 && <span className="text-stone-500"> ×{node.entry.quantity}</span>}
        </button>
        <span className="text-[10px] text-stone-600 tabular-nums shrink-0">
          {formatGrams(Number(node.item.metadata?.core?.weight_g ?? 0) * (node.entry.quantity || 1))}
        </span>
        {/* A pack is gear you wear and a place to put gear, so it needs a way in from
            wherever it is equipped. */}
        {isContainer && (
          <button
            onClick={() => onZoom(node)}
            className="text-stone-600 hover:text-white shrink-0"
            title={`Open (${node.children?.filter((c) => !c.hidden).length ?? 0} inside)`}
          >
            <Maximize2 className="w-3 h-3" />
          </button>
        )}
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
    );
  }
}

/** Says a slot has gear the tag filter is hiding, so a filtered slot never reads as empty. */
export function HiddenNote({ count }: { count: number }) {
  return (
    <div
      className="mt-1 flex items-center gap-1 text-[10px] text-stone-600"
      title="Hidden by the tag filter. Clear the filter to see everything."
    >
      <EyeOff className="w-3 h-3" />
      {count} hidden by filter
    </div>
  );
}
