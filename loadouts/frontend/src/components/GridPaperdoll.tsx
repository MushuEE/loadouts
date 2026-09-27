import { useId, useMemo, useState, type ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import type { PaperdollLayout, ResolvedEntry, SlotDefinition } from '../api/types';
import { FIGURES, TONE_FILL } from '../paperdoll/figures';
import { TILE, blockIndexAt, cellOwners, centroid, decodeRuns, footprint } from '../paperdoll/grid';
import { ItemThumb } from './ItemThumb';
import { PaperdollSlot } from './Paperdoll';

/** A region to light up on the figures: some cells, clipped to the silhouette under them. */
export interface Highlight {
  cells: number[];
  fill: string;
  opacity: number;
  /** Outline each cell, so the grid under the smooth figure shows through. */
  outline?: boolean;
}

/**
 * Draws a layout's figure blocks into an SVG whose user space is the canvas grid, one unit
 * per cell. Highlights are drawn as whole cells and then clipped to the figure they sit on,
 * so a torso binding lights the torso and not the square of air around it.
 *
 * Tiles are not drawn here. They are HTML cards laid over the canvas by the caller, since
 * they hold interactive content rather than artwork.
 */
export function FigureLayer({
  layout,
  highlights,
  grid,
}: {
  layout: PaperdollLayout;
  highlights: Highlight[];
  /** Draw every cell of every block: the editor's view. */
  grid?: boolean;
}) {
  const uid = useId().replace(/:/g, '');
  return (
    <>
      <defs>
        {/* userSpaceOnUse in each figure's own cell space, so every figure shades head to
            toe the same way instead of each limb getting its own gradient. */}
        <linearGradient id="pdMannequin" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="3" y2="16">
          <stop offset="0" stopColor="#d6d3d1" />
          <stop offset="1" stopColor="#8f8a86" />
        </linearGradient>
      </defs>
      {layout.blocks.map((block, i) => {
        const fig = FIGURES[block.block];
        if (!fig) return null;
        const clipId = `${uid}-fig-${i}`;
        const local = (c: number) => ({ x: (c % layout.cols) - block.col, y: Math.floor(c / layout.cols) - block.row });
        const onBlock = (c: number) => {
          const { x, y } = local(c);
          return x >= 0 && y >= 0 && x < fig.cols && y < fig.rows;
        };
        return (
          <g key={i} transform={`translate(${block.col} ${block.row})`}>
            <clipPath id={clipId}>
              {fig.shapes.map((s, j) => (
                <path key={j} d={s.d} clipRule="evenodd" />
              ))}
            </clipPath>
            {fig.shapes.map((s, j) => (
              <path key={j} d={s.d} fill={TONE_FILL[s.tone]} fillRule="evenodd" />
            ))}
            <g clipPath={`url(#${clipId})`}>
              {highlights.map((h, k) =>
                h.cells.filter(onBlock).map((c) => {
                  const { x, y } = local(c);
                  return (
                    <rect
                      key={`${k}-${c}`}
                      x={x}
                      y={y}
                      // A hair oversize, so neighbouring cells do not leave hairline seams.
                      width={1.02}
                      height={1.02}
                      fill={h.fill}
                      fillOpacity={h.opacity}
                      stroke={h.outline ? h.fill : 'none'}
                      strokeOpacity={h.outline ? 0.9 : 0}
                      strokeWidth={0.05}
                    />
                  );
                }),
              )}
            </g>
            {grid && (
              <g stroke="#a8a29e" strokeOpacity={0.18} strokeWidth={0.03}>
                {Array.from({ length: fig.cols + 1 }, (_, x) => (
                  <line key={`v${x}`} x1={x} y1={0} x2={x} y2={fig.rows} />
                ))}
                {Array.from({ length: fig.rows + 1 }, (_, y) => (
                  <line key={`h${y}`} x1={0} y1={y} x2={fig.cols} y2={y} />
                ))}
              </g>
            )}
          </g>
        );
      })}
    </>
  );
}

/** Converts a pointer position over the canvas into a cell, or null off the canvas. */
export function cellFromPointer(e: React.MouseEvent, layout: PaperdollLayout): { col: number; row: number } | null {
  const rect = (e.currentTarget as Element).getBoundingClientRect();
  const col = Math.floor(((e.clientX - rect.left) / rect.width) * layout.cols);
  const row = Math.floor(((e.clientY - rect.top) / rect.height) * layout.rows);
  if (col < 0 || row < 0 || col >= layout.cols || row >= layout.rows) return null;
  return { col, row };
}

/** Percent box for a block, for laying HTML over the SVG canvas. */
export function blockStyle(layout: PaperdollLayout, block: { col: number; row: number; cols: number; rows: number }) {
  return {
    left: `${(block.col / layout.cols) * 100}%`,
    top: `${(block.row / layout.rows) * 100}%`,
    width: `${(block.cols / layout.cols) * 100}%`,
    height: `${(block.rows / layout.rows) * 100}%`,
  };
}

/**
 * The template author's paperdoll: slots placed on figures and tiles by an explicit layout,
 * rather than inferred from categories (that fallback is the plain Paperdoll).
 *
 * Figure-bound slots get frames flanking the canvas, each on the side of the figure its
 * cells lean towards. Hovering a frame lights its cells on the figure, and hovering the
 * figure finds the frame, so the link works in both directions. Tile-bound slots are the
 * tile: the card on the canvas is where you add and see the items.
 */
export function GridPaperdoll({
  layout,
  slots,
  entries,
  readOnly,
  selectedEntryId,
  onPick,
  onRemove,
  onSelect,
  action,
}: {
  layout: PaperdollLayout;
  slots: SlotDefinition[];
  entries: ResolvedEntry[];
  readOnly: boolean;
  selectedEntryId: string | null;
  onPick: (slot: SlotDefinition) => void;
  onRemove: (node: ResolvedEntry) => void;
  onSelect: (entryId: string | null) => void;
  /** Rendered in the header, for the editor's entry point. */
  action?: ReactNode;
}) {
  const [hovered, setHovered] = useState<string | null>(null);
  const slotById = useMemo(() => new Map(slots.map((s) => [s.id, s])), [slots]);
  const owners = useMemo(() => cellOwners(layout), [layout]);
  const occupantsOf = (slotId: string) => entries.filter((e) => e.entry.slot_id === slotId);

  const { tiles, left, right, cellsOf } = useMemo(() => {
    const cellsOf = new Map(layout.bindings.map((b) => [b.slot_id, decodeRuns(b.cells)]));
    const tiles = layout.blocks
      .filter((b) => b.block === TILE)
      .map((b) => ({ ...b, ...footprint(b), slotId: owners.get(b.row * layout.cols + b.col) }));

    // A figure slot goes on the side of the figure its cells lean towards. Symmetric ones
    // (a torso, both feet) fill in top to bottom on whichever side is shorter, which keeps
    // the columns balanced and roughly level with what they point at.
    const figureSlots = layout.bindings
      .filter((b) => slotById.has(b.slot_id))
      .map((b) => {
        const cells = cellsOf.get(b.slot_id)!;
        const c = centroid(cells, layout.cols);
        const block = layout.blocks[blockIndexAt(layout, Math.floor(c.x), Math.floor(c.y))] ?? layout.blocks[0];
        return { slotId: b.slot_id, c, block, lean: block ? c.x - (block.col + footprint(block).cols / 2) : 0 };
      })
      .filter((s) => s.block && s.block.block !== TILE)
      .sort((a, b) => a.c.y - b.c.y);
    const left: string[] = [];
    const right: string[] = [];
    for (const s of figureSlots) {
      if (s.lean < -0.75) left.push(s.slotId);
      else if (s.lean > 0.75) right.push(s.slotId);
      else (left.length <= right.length ? left : right).push(s.slotId);
    }
    return { tiles, left, right, cellsOf };
  }, [layout, owners, slotById]);

  const highlights: Highlight[] = [];
  for (const [slotId, cells] of cellsOf) {
    if (!slotById.has(slotId)) continue;
    const occupants = occupantsOf(slotId);
    const selected = occupants.some((n) => n.entry.id === selectedEntryId);
    if (selected) highlights.push({ cells, fill: '#38bdf8', opacity: 0.5, outline: true });
    else if (hovered === slotId)
      highlights.push(
        occupants.length
          ? { cells, fill: '#fbbf24', opacity: 0.55, outline: true }
          : { cells, fill: '#fafaf9', opacity: 0.25, outline: true },
      );
    // Equipped regions keep a faint warm cast, so the figure reads as dressed at a glance.
    else if (occupants.length) highlights.push({ cells, fill: '#f59e0b', opacity: 0.08 });
  }

  const hasFigures = layout.blocks.some((b) => b.block !== TILE);

  const figureSlotAt = (e: React.MouseEvent) => {
    const cell = cellFromPointer(e, layout);
    if (!cell) return null;
    const slotId = owners.get(cell.row * layout.cols + cell.col);
    const block = layout.blocks[blockIndexAt(layout, cell.col, cell.row)];
    return slotId && block && block.block !== TILE && slotById.has(slotId) ? slotId : null;
  };

  const column = (ids: string[]) => (
    <div className="flex flex-col gap-2.5 min-w-0">
      {ids.map((id) => (
        <PaperdollSlot
          key={id}
          slot={slotById.get(id)!}
          occupants={occupantsOf(id)}
          readOnly={readOnly}
          hovered={hovered === id}
          selectedEntryId={selectedEntryId}
          onHover={setHovered}
          onPick={() => onPick(slotById.get(id)!)}
          onRemove={onRemove}
          onSelect={onSelect}
        />
      ))}
    </div>
  );

  const canvas = (
    <div className="relative w-full" style={{ aspectRatio: `${layout.cols} / ${layout.rows}` }}>
      <svg
        viewBox={`0 0 ${layout.cols} ${layout.rows}`}
        className={`absolute inset-0 w-full h-full ${hovered ? 'cursor-pointer' : ''}`}
        onMouseMove={(e) => setHovered(figureSlotAt(e))}
        onMouseLeave={() => setHovered(null)}
        onClick={(e) => {
          const slotId = figureSlotAt(e);
          if (!slotId) return;
          const first = occupantsOf(slotId)[0];
          if (first) onSelect(first.entry.id === selectedEntryId ? null : first.entry.id);
          else if (!readOnly) onPick(slotById.get(slotId)!);
        }}
      >
        <FigureLayer layout={layout} highlights={highlights} />
      </svg>
      {tiles.map((t, i) => {
        const slot = t.slotId ? slotById.get(t.slotId) : undefined;
        return (
          <div key={i} className="absolute p-[3px]" style={blockStyle(layout, t)}>
            {slot ? (
              <TileCard
                slot={slot}
                occupants={occupantsOf(slot.id)}
                readOnly={readOnly}
                selectedEntryId={selectedEntryId}
                onPick={() => onPick(slot)}
                onRemove={onRemove}
                onSelect={onSelect}
              />
            ) : (
              <div className="w-full h-full rounded-lg border border-dashed border-stone-800" />
            )}
          </div>
        );
      })}
    </div>
  );

  return (
    <div className="mb-8 p-5 rounded-xl bg-stone-900/60 border border-stone-800">
      <div className="flex items-center justify-between gap-3 mb-4">
        <span className="text-[10px] font-bold uppercase tracking-widest text-stone-500">Equipped</span>
        <div className="flex items-center gap-3">
          {hasFigures && <span className="text-[10px] text-stone-600">Hover the figure or a slot to link them</span>}
          {action}
        </div>
      </div>

      {hasFigures ? (
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.7fr)_minmax(0,1fr)] gap-4 items-center">
          {column(left)}
          {canvas}
          {column(right)}
        </div>
      ) : (
        <div className="max-w-xl mx-auto">{canvas}</div>
      )}
    </div>
  );
}

/**
 * A tile is a slot with no body part: a meal, a sleep system. It shows its items directly,
 * as a stack of thumbnails when space is tight.
 */
function TileCard({
  slot,
  occupants,
  readOnly,
  selectedEntryId,
  onPick,
  onRemove,
  onSelect,
}: {
  slot: SlotDefinition;
  occupants: ResolvedEntry[];
  readOnly: boolean;
  selectedEntryId: string | null;
  onPick: () => void;
  onRemove: (node: ResolvedEntry) => void;
  onSelect: (entryId: string | null) => void;
}) {
  const capacity = slot.max_items === -1 ? Infinity : Math.max(slot.max_items, 1);
  const full = occupants.length >= capacity;
  const holdsSelection = occupants.some((n) => n.entry.id === selectedEntryId);
  return (
    <div
      className={`w-full h-full rounded-lg border p-2 flex flex-col overflow-hidden ${
        holdsSelection
          ? 'border-sky-400/70 bg-sky-400/5'
          : occupants.length
            ? 'border-stone-700 bg-stone-900'
            : 'border-dashed border-stone-700 bg-stone-900/40'
      }`}
    >
      <div className="flex items-center justify-between gap-2 shrink-0">
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
      {occupants.length === 0 ? (
        <div className="flex-1 flex items-center justify-center text-[11px] text-stone-600 italic">Empty</div>
      ) : (
        <div className="flex-1 min-h-0 mt-1.5 flex flex-wrap content-start gap-1.5 overflow-hidden">
          {occupants.map((node) => {
            const selected = node.entry.id === selectedEntryId;
            return (
              <div key={node.entry.id} className="group flex items-center gap-1 max-w-full">
                <button
                  onClick={() => onSelect(selected ? null : node.entry.id)}
                  title={node.item.name}
                  className={`flex items-center gap-1.5 min-w-0 rounded-md pr-1.5 ${
                    selected ? 'bg-sky-400/15 text-sky-200' : 'text-stone-300 hover:text-white hover:bg-stone-800'
                  }`}
                >
                  <ItemThumb imageUrl={node.item.image_url} category={node.item.category} size="w-6 h-6" iconSize="w-3.5 h-3.5" />
                  <span className="text-[11px] truncate">{node.item.name}</span>
                </button>
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
          })}
        </div>
      )}
    </div>
  );
}
