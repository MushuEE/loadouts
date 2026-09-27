import type { PaperdollBlock, PaperdollLayout } from '../api/types';
import { FIGURES } from './figures';

/**
 * Cell arithmetic for grid paperdolls. Cells are indexed row-major (row * cols + col) and
 * stored as runs, mirroring core/paperdoll.go on the server, which has the final say.
 */

export const TILE = 'tile';
export const MAX_SIDE = 48;

export type Runs = [number, number][];

export function decodeRuns(runs: Runs): number[] {
  const out: number[] = [];
  for (const [start, len] of runs) for (let c = start; c < start + len; c++) out.push(c);
  return out;
}

export function encodeCells(cells: Iterable<number>): Runs {
  const sorted = [...new Set(cells)].sort((a, b) => a - b);
  const runs: Runs = [];
  for (const c of sorted) {
    const last = runs[runs.length - 1];
    if (last && c === last[0] + last[1]) last[1]++;
    else runs.push([c, 1]);
  }
  return runs;
}

export function footprint(block: PaperdollBlock): { cols: number; rows: number } {
  const fig = FIGURES[block.block];
  return fig ? { cols: fig.cols, rows: fig.rows } : { cols: block.cols ?? 0, rows: block.rows ?? 0 };
}

export function blockContains(block: PaperdollBlock, col: number, row: number): boolean {
  const { cols, rows } = footprint(block);
  return col >= block.col && col < block.col + cols && row >= block.row && row < block.row + rows;
}

export function blocksOverlap(a: PaperdollBlock, b: PaperdollBlock): boolean {
  const fa = footprint(a);
  const fb = footprint(b);
  return a.col < b.col + fb.cols && b.col < a.col + fa.cols && a.row < b.row + fb.rows && b.row < a.row + fa.rows;
}

export function blockIndexAt(layout: PaperdollLayout, col: number, row: number): number {
  return layout.blocks.findIndex((b) => blockContains(b, col, row));
}

/** Every cell in a block's footprint. */
export function blockCells(block: PaperdollBlock, canvasCols: number): number[] {
  const { cols, rows } = footprint(block);
  const out: number[] = [];
  for (let r = block.row; r < block.row + rows; r++)
    for (let c = block.col; c < block.col + cols; c++) out.push(r * canvasCols + c);
  return out;
}

/** Which slot owns each cell. */
export function cellOwners(layout: PaperdollLayout): Map<number, string> {
  const owners = new Map<number, string>();
  for (const b of layout.bindings) for (const c of decodeRuns(b.cells)) owners.set(c, b.slot_id);
  return owners;
}

/** Whether a block fits on the canvas without overlapping any other (skipping `ignore`). */
export function fits(layout: PaperdollLayout, block: PaperdollBlock, ignore = -1): boolean {
  const { cols, rows } = footprint(block);
  if (block.col < 0 || block.row < 0 || block.col + cols > layout.cols || block.row + rows > layout.rows) return false;
  return layout.blocks.every((other, i) => i === ignore || !blocksOverlap(block, other));
}

/** The first top-left position, scanning row-major, where a block of this shape fits. */
export function firstFit(layout: PaperdollLayout, block: PaperdollBlock): PaperdollBlock | null {
  for (let row = 0; row < layout.rows; row++)
    for (let col = 0; col < layout.cols; col++) {
      const placed = { ...block, col, row };
      if (fits(layout, placed)) return placed;
    }
  return null;
}

/** Mean position of a set of cells, in cell units. */
export function centroid(cells: number[], canvasCols: number): { x: number; y: number } {
  if (cells.length === 0) return { x: 0, y: 0 };
  let x = 0;
  let y = 0;
  for (const c of cells) {
    x += (c % canvasCols) + 0.5;
    y += Math.floor(c / canvasCols) + 0.5;
  }
  return { x: x / cells.length, y: y / cells.length };
}
