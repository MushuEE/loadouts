import { useMemo, useRef, useState } from 'react';
import { Brush, Eraser, Move, Plus, Trash2, X } from 'lucide-react';
import { api } from '../api/client';
import type { PaperdollBlock, PaperdollLayout, TemplateDetail } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { FIGURES } from '../paperdoll/figures';
import {
  MAX_SIDE,
  TILE,
  blockCells,
  blockContains,
  blockIndexAt,
  decodeRuns,
  encodeCells,
  firstFit,
  fits,
  footprint,
} from '../paperdoll/grid';
import { ErrorNote, Spinner } from './ui';
import { FigureLayer, cellFromPointer, type Highlight } from './GridPaperdoll';

const SLOT_COLORS = ['#f97316', '#38bdf8', '#a3e635', '#f472b6', '#facc15', '#34d399', '#c084fc', '#fb7185', '#60a5fa', '#2dd4bf'];

type Mode = 'paint' | 'erase' | 'arrange';

/**
 * Lays a template's slots out on a grid: drop figures and tiles on a canvas, then paint the
 * cells each slot owns.
 *
 * Saving publishes a new template version with the same slots, so loadouts pinned to the
 * old one are untouched until their owners move up. The server re-checks everything here,
 * including that only site admins place figures; the editor just refuses the obvious
 * mistakes early, where they are cheap to explain.
 *
 * Painting is mouse-only for now. Touch would fight with scrolling, and it is not worth
 * solving until authors other than site admins arrive.
 */
export function PaperdollEditor({
  templateId,
  isSiteAdmin,
  onClose,
  onSaved,
}: {
  templateId: string;
  isSiteAdmin: boolean;
  onClose: () => void;
  onSaved: (detail: TemplateDetail) => void;
}) {
  // Always the latest version: that is what the new version's slots will be copied from.
  const latest = useAsync<TemplateDetail>(() => api.getTemplate(templateId), [templateId]);

  return (
    <div className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-6">
      <div className="w-full max-w-6xl h-[90vh] bg-stone-950 border border-stone-800 rounded-2xl flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-stone-800 shrink-0">
          <div>
            <h2 className="text-lg font-medium text-white">Edit paperdoll</h2>
            <p className="text-[11px] text-stone-500">
              {latest.data
                ? `${latest.data.template.name} · saving publishes v${latest.data.template.latest_version + 1}`
                : 'Loading template…'}
            </p>
          </div>
          <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>
        {/* A grid of hand-sized cells does not survive a phone screen, and painting it
            with a finger would fight the page scroll. */}
        <div className="md:hidden p-6 text-sm text-stone-400">
          The paperdoll editor needs a larger screen and a mouse.
        </div>
        <div className="hidden md:flex flex-1 min-h-0">
          {latest.loading && <Spinner />}
          {latest.error && (
            <div className="p-6">
              <ErrorNote message={latest.error} />
            </div>
          )}
          {latest.data && (
            <EditorBody detail={latest.data} isSiteAdmin={isSiteAdmin} onSaved={onSaved} />
          )}
        </div>
      </div>
    </div>
  );
}

function EditorBody({
  detail,
  isSiteAdmin,
  onSaved,
}: {
  detail: TemplateDetail;
  isSiteAdmin: boolean;
  onSaved: (detail: TemplateDetail) => void;
}) {
  const slots = detail.version.slots;
  const existing = detail.version.paperdoll;

  const [size, setSize] = useState({ cols: existing?.cols ?? 16, rows: existing?.rows ?? 16 });
  const [blocks, setBlocks] = useState<PaperdollBlock[]>(existing?.blocks ?? []);
  const [cells, setCells] = useState<Map<string, Set<number>>>(
    () => new Map((existing?.bindings ?? []).map((b) => [b.slot_id, new Set(decodeRuns(b.cells))])),
  );
  const [mode, setMode] = useState<Mode>(existing ? 'paint' : 'arrange');
  const [activeSlot, setActiveSlot] = useState(slots[0]?.id ?? '');
  const [selectedBlock, setSelectedBlock] = useState<number | null>(null);
  const [tile, setTile] = useState({ cols: 4, rows: 4, slot: slots[0]?.id ?? '' });
  const [notice, setNotice] = useState<string | null>(null);
  const [changelog, setChangelog] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const drag = useRef<{ kind: Mode; block?: number; grab?: { col: number; row: number } } | null>(null);

  const layout: PaperdollLayout = useMemo(
    () => ({
      cols: size.cols,
      rows: size.rows,
      blocks,
      // Slot order, so the saved JSON is stable no matter the order things were painted.
      bindings: slots
        .filter((s) => (cells.get(s.id)?.size ?? 0) > 0)
        .map((s) => ({ slot_id: s.id, cells: encodeCells(cells.get(s.id)!) })),
    }),
    [size, blocks, cells, slots],
  );

  const colorOf = useMemo(() => new Map(slots.map((s, i) => [s.id, SLOT_COLORS[i % SLOT_COLORS.length]])), [slots]);
  const slotName = (id: string) => slots.find((s) => s.id === id)?.name ?? id;
  const ownerOf = (cell: number) => {
    for (const [slot, set] of cells) if (set.has(cell)) return slot;
    return null;
  };
  const hasFigures = blocks.some((b) => b.block !== TILE);

  function updateCells(fn: (next: Map<string, Set<number>>) => void) {
    setCells((prev) => {
      const next = new Map([...prev].map(([k, v]) => [k, new Set(v)]));
      fn(next);
      return next;
    });
  }

  function paintAt(col: number, row: number, kind: Mode) {
    const cell = row * size.cols + col;
    if (kind === 'erase') {
      updateCells((next) => next.forEach((set) => set.delete(cell)));
      return;
    }
    if (!activeSlot) return;
    if (blockIndexAt(layout, col, row) < 0) {
      setNotice('Paint on a figure or a tile. Bare canvas cannot hold a slot.');
      return;
    }
    const owner = ownerOf(cell);
    if (owner && owner !== activeSlot) {
      // One cell, one slot. Layering is several items in one slot, not stacked slots.
      setNotice(`That cell belongs to "${slotName(owner)}". Erase it first to give it to another slot.`);
      return;
    }
    if (!owner) updateCells((next) => next.set(activeSlot, new Set([...(next.get(activeSlot) ?? []), cell])));
  }

  /** Moves a block, carrying the cells painted on it. */
  function moveBlock(index: number, col: number, row: number) {
    const block = blocks[index];
    const moved = { ...block, col, row };
    if ((block.col === col && block.row === row) || !fits(layout, moved, index)) return;
    const dc = col - block.col;
    const dr = row - block.row;
    updateCells((next) =>
      next.forEach((set, slot) => {
        const shifted = [...set].map((c) => {
          const cc = c % size.cols;
          const rr = Math.floor(c / size.cols);
          return blockContains(block, cc, rr) ? (rr + dr) * size.cols + cc + dc : c;
        });
        next.set(slot, new Set(shifted));
      }),
    );
    setBlocks(blocks.map((b, i) => (i === index ? moved : b)));
  }

  function onPointerDown(e: React.MouseEvent) {
    const cell = cellFromPointer(e, layout);
    if (!cell) return;
    setNotice(null);
    if (mode === 'arrange') {
      const index = blockIndexAt(layout, cell.col, cell.row);
      setSelectedBlock(index >= 0 ? index : null);
      if (index >= 0) {
        drag.current = { kind: 'arrange', block: index, grab: { col: cell.col - blocks[index].col, row: cell.row - blocks[index].row } };
      }
      return;
    }
    drag.current = { kind: mode };
    paintAt(cell.col, cell.row, mode);
  }

  function onPointerMove(e: React.MouseEvent) {
    const d = drag.current;
    if (!d || e.buttons !== 1) return;
    const cell = cellFromPointer(e, layout);
    if (!cell) return;
    if (d.kind === 'arrange' && d.block !== undefined && d.grab) moveBlock(d.block, cell.col - d.grab.col, cell.row - d.grab.row);
    else paintAt(cell.col, cell.row, d.kind);
  }

  function addBlock(block: PaperdollBlock, bindTo?: string) {
    const placed = firstFit(layout, block);
    if (!placed) {
      setNotice('No room for that. Grow the canvas or move something.');
      return;
    }
    setBlocks([...blocks, placed]);
    setSelectedBlock(blocks.length);
    if (bindTo) {
      updateCells((next) => next.set(bindTo, new Set([...(next.get(bindTo) ?? []), ...blockCells(placed, size.cols)])));
    }
  }

  function deleteBlock(index: number) {
    const doomed = new Set(blockCells(blocks[index], size.cols));
    updateCells((next) => next.forEach((set) => doomed.forEach((c) => set.delete(c))));
    setBlocks(blocks.filter((_, i) => i !== index));
    setSelectedBlock(null);
  }

  /** Changing the width re-indexes every cell, since indexes are row-major. */
  function resize(cols: number, rows: number) {
    cols = Math.max(1, Math.min(MAX_SIDE, cols || 1));
    rows = Math.max(1, Math.min(MAX_SIDE, rows || 1));
    const next = { cols, rows, blocks, bindings: [] };
    if (!blocks.every((b, i) => fits(next, b, i))) {
      setNotice('Something would fall off the canvas. Move it first.');
      return;
    }
    updateCells((m) =>
      m.forEach((set, slot) => m.set(slot, new Set([...set].map((c) => Math.floor(c / size.cols) * cols + (c % size.cols))))),
    );
    setSize({ cols, rows });
  }

  async function save(remove = false) {
    if (remove && !window.confirm('Remove the paperdoll? Loadouts on the new version fall back to the automatic figure.')) return;
    setSaving(true);
    setError(null);
    try {
      const saved = await api.publishPaperdoll(detail.template.id, {
        paperdoll: remove ? null : layout,
        changelog: changelog.trim() || undefined,
      });
      onSaved(saved);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  }

  const highlights: Highlight[] = layout.bindings.map((b) => ({
    cells: decodeRuns(b.cells),
    fill: colorOf.get(b.slot_id)!,
    opacity: b.slot_id === activeSlot && mode !== 'arrange' ? 0.7 : 0.35,
    outline: b.slot_id === activeSlot,
  }));
  const tileCells = (block: PaperdollBlock) => {
    const set = new Set(blockCells(block, size.cols));
    return layout.bindings.flatMap((b) =>
      decodeRuns(b.cells)
        .filter((c) => set.has(c))
        .map((c) => ({ c, slot: b.slot_id })),
    );
  };

  const tool = (m: Mode, Icon: typeof Brush, label: string) => (
    <button
      onClick={() => setMode(m)}
      className={`flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-md text-xs ${
        mode === m ? 'bg-stone-700 text-white' : 'text-stone-400 hover:text-white'
      }`}
    >
      <Icon className="w-3.5 h-3.5" /> {label}
    </button>
  );

  const numberInput = (value: number, onChange: (n: number) => void, title: string) => (
    <input
      type="number"
      min={1}
      max={MAX_SIDE}
      value={value}
      title={title}
      onChange={(e) => onChange(parseInt(e.target.value, 10))}
      className="w-14 min-w-0 bg-stone-900 border border-stone-800 rounded-md px-2 py-1 text-xs text-white outline-none focus:border-orange-500"
    />
  );

  return (
    <>
      <aside className="w-72 border-r border-stone-800 overflow-y-auto p-5 space-y-6 shrink-0">
        <div className="flex gap-1 p-1 rounded-lg bg-stone-900 border border-stone-800">
          {tool('arrange', Move, 'Arrange')}
          {tool('paint', Brush, 'Paint')}
          {tool('erase', Eraser, 'Erase')}
        </div>

        <section>
          <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500 mb-2">Slots</div>
          <div className="space-y-1">
            {slots.map((s) => {
              const count = cells.get(s.id)?.size ?? 0;
              const active = s.id === activeSlot;
              return (
                <div
                  key={s.id}
                  className={`group flex items-center gap-2 px-2 py-1.5 rounded-md cursor-pointer ${
                    active ? 'bg-stone-800' : 'hover:bg-stone-900'
                  }`}
                  onClick={() => {
                    setActiveSlot(s.id);
                    if (mode === 'arrange') setMode('paint');
                  }}
                >
                  <span className="w-3 h-3 rounded-sm shrink-0" style={{ background: colorOf.get(s.id) }} />
                  <span className={`text-xs truncate flex-1 ${active ? 'text-white' : 'text-stone-400'}`}>{s.name}</span>
                  <span className="text-[10px] text-stone-600 tabular-nums">{count || ''}</span>
                  {count > 0 && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        updateCells((next) => next.delete(s.id));
                      }}
                      className="opacity-0 group-hover:opacity-100 text-stone-600 hover:text-red-400"
                      title="Clear this slot's cells"
                    >
                      <X className="w-3 h-3" />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
          <p className="text-[10px] text-stone-600 mt-2 leading-snug">
            Unpainted slots stay in the grid under the paperdoll. A cell belongs to one slot.
          </p>
        </section>

        <section>
          <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500 mb-2">Figures</div>
          {isSiteAdmin ? (
            <div className="grid grid-cols-2 gap-1.5">
              {Object.values(FIGURES).map((fig) => (
                <button
                  key={fig.id}
                  onClick={() => addBlock({ block: fig.id, col: 0, row: 0 })}
                  className="flex items-center gap-1.5 px-2 py-1.5 rounded-md bg-stone-900 border border-stone-800 hover:border-stone-600 text-xs text-stone-300"
                >
                  <Plus className="w-3 h-3" /> {fig.label}
                  <span className="ml-auto text-[10px] text-stone-600">
                    {fig.cols}×{fig.rows}
                  </span>
                </button>
              ))}
            </div>
          ) : (
            <p className="text-[11px] text-stone-600 leading-snug">Figures are placed by site admins. Tiles are yours to lay out.</p>
          )}
        </section>

        <section>
          <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500 mb-2">Tile</div>
          <div className="flex items-center gap-1.5">
            {numberInput(tile.cols, (n) => setTile({ ...tile, cols: n || 1 }), 'Width in cells')}
            <span className="text-stone-600 text-xs">×</span>
            {numberInput(tile.rows, (n) => setTile({ ...tile, rows: n || 1 }), 'Height in cells')}
            <select
              value={tile.slot}
              onChange={(e) => setTile({ ...tile, slot: e.target.value })}
              className="flex-1 min-w-0 bg-stone-900 border border-stone-800 rounded-md px-1.5 py-1 text-xs text-stone-300 outline-none"
            >
              {slots.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
          <button
            onClick={() => addBlock({ block: TILE, col: 0, row: 0, cols: tile.cols, rows: tile.rows }, tile.slot)}
            className="mt-2 w-full flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-md bg-stone-900 border border-stone-800 hover:border-stone-600 text-xs text-stone-300"
          >
            <Plus className="w-3 h-3" /> Add tile
          </button>
        </section>

        <section>
          <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500 mb-2">Canvas</div>
          <div className="flex items-center gap-1.5">
            {numberInput(size.cols, (n) => resize(n, size.rows), 'Columns')}
            <span className="text-stone-600 text-xs">×</span>
            {numberInput(size.rows, (n) => resize(size.cols, n), 'Rows')}
            <span className="text-[10px] text-stone-600">cells</span>
          </div>
        </section>

        {selectedBlock !== null && blocks[selectedBlock] && (
          <section className="p-3 rounded-lg bg-stone-900 border border-stone-800">
            <div className="flex items-center justify-between">
              <span className="text-xs text-stone-300">
                {FIGURES[blocks[selectedBlock].block]?.label ?? 'Tile'} at {blocks[selectedBlock].col},{' '}
                {blocks[selectedBlock].row}
              </span>
              <button
                onClick={() => deleteBlock(selectedBlock)}
                disabled={blocks[selectedBlock].block !== TILE && !isSiteAdmin}
                className="text-stone-500 hover:text-red-400 disabled:opacity-30"
                title="Remove block and its painted cells"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
            <p className="text-[10px] text-stone-600 mt-1">Drag it in Arrange mode; its painted cells move with it.</p>
          </section>
        )}
      </aside>

      <div className="flex-1 min-w-0 flex flex-col">
        <div className="flex-1 min-h-0 p-6 flex items-center justify-center overflow-auto">
          <div
            className="relative bg-stone-900/40 rounded-md"
            style={{
              aspectRatio: `${size.cols} / ${size.rows}`,
              width: `min(100%, calc((90vh - 13rem) * ${size.cols / size.rows}))`,
            }}
          >
            <svg
              viewBox={`0 0 ${size.cols} ${size.rows}`}
              className={`absolute inset-0 w-full h-full select-none ${mode === 'arrange' ? 'cursor-move' : 'cursor-crosshair'}`}
              onMouseDown={onPointerDown}
              onMouseMove={onPointerMove}
              onMouseUp={() => (drag.current = null)}
              onMouseLeave={() => (drag.current = null)}
            >
              {/* Bare canvas, as a dot per cell: present enough to count, quiet enough to
                  not compete with the blocks. */}
              <g fill="#57534e">
                {Array.from({ length: size.cols * size.rows }, (_, i) => (
                  <circle key={i} cx={(i % size.cols) + 0.5} cy={Math.floor(i / size.cols) + 0.5} r={0.05} />
                ))}
              </g>
              {blocks.map((b, i) =>
                b.block === TILE ? (
                  <g key={i}>
                    <rect
                      x={b.col + 0.05}
                      y={b.row + 0.05}
                      width={footprint(b).cols - 0.1}
                      height={footprint(b).rows - 0.1}
                      rx={0.3}
                      fill="#292524"
                      stroke="#57534e"
                      strokeWidth={0.05}
                    />
                    {tileCells(b).map(({ c, slot }) => (
                      <rect
                        key={c}
                        x={c % size.cols}
                        y={Math.floor(c / size.cols)}
                        width={1.02}
                        height={1.02}
                        fill={colorOf.get(slot)}
                        fillOpacity={slot === activeSlot && mode !== 'arrange' ? 0.55 : 0.28}
                      />
                    ))}
                  </g>
                ) : null,
              )}
              <FigureLayer layout={layout} highlights={highlights} grid />
              {selectedBlock !== null && blocks[selectedBlock] && (
                <rect
                  x={blocks[selectedBlock].col}
                  y={blocks[selectedBlock].row}
                  width={footprint(blocks[selectedBlock]).cols}
                  height={footprint(blocks[selectedBlock]).rows}
                  fill="none"
                  stroke="#f97316"
                  strokeWidth={0.08}
                  strokeDasharray="0.3 0.2"
                  pointerEvents="none"
                />
              )}
            </svg>
          </div>
        </div>

        <div className="border-t border-stone-800 px-6 py-3 flex items-center gap-3 shrink-0">
          <div className="flex-1 min-w-0 text-[11px]">
            {error ? (
              <span className="text-red-400">{error}</span>
            ) : notice ? (
              <span className="text-amber-400">{notice}</span>
            ) : hasFigures && !isSiteAdmin ? (
              <span className="text-amber-400">This layout has figures, so only a site admin can save it.</span>
            ) : (
              <span className="text-stone-600">
                {mode === 'arrange' ? 'Drag blocks to move them.' : `Painting "${slotName(activeSlot)}". Drag across cells.`}
              </span>
            )}
          </div>
          <input
            value={changelog}
            onChange={(e) => setChangelog(e.target.value)}
            placeholder="Changelog (optional)"
            className="w-56 bg-stone-900 border border-stone-800 rounded-md px-2.5 py-1.5 text-xs text-white outline-none focus:border-orange-500"
          />
          {existing && (
            <button
              onClick={() => save(true)}
              disabled={saving}
              className="px-3 py-1.5 rounded-md text-xs text-stone-400 hover:text-red-400 disabled:opacity-50"
            >
              Remove paperdoll
            </button>
          )}
          <button
            onClick={() => save()}
            disabled={saving || (hasFigures && !isSiteAdmin)}
            className="px-4 py-1.5 rounded-md bg-orange-600 hover:bg-orange-500 text-sm font-medium text-white disabled:opacity-50"
          >
            {saving ? 'Publishing…' : 'Publish version'}
          </button>
        </div>
      </div>
    </>
  );
}

