/**
 * Prefabricated paperdoll figures, drawn in *cell units*.
 *
 * Each figure's coordinate space is its footprint on the grid: a 10x16 figure is drawn in a
 * 10x16 box, so one SVG unit is one cell. Dropping it at (col, row) is then a plain
 * translate, and a binding's cells line up with the artwork without any scaling maths.
 *
 * Every shape is a filled path, never a stroke. The same shapes are reused as the clip path
 * for highlights, and clip paths ignore strokes, so a limb drawn as a thick line would light
 * up as nothing. Limbs are therefore capsules between two joints.
 *
 * Footprints must match core.PaperdollFigures on the server, which validates placements.
 */

export type Tone = 'body' | 'gear' | 'frame';

export interface FigureShape {
  d: string;
  tone: Tone;
}

export interface Figure {
  id: string;
  label: string;
  cols: number;
  rows: number;
  shapes: FigureShape[];
}

const f = (n: number) => +n.toFixed(3);

/** A circle as a path, so every shape can share one renderer. */
function circle(cx: number, cy: number, r: number): string {
  return `M${f(cx - r)} ${f(cy)}a${f(r)} ${f(r)} 0 1 0 ${f(2 * r)} 0a${f(r)} ${f(r)} 0 1 0 ${f(-2 * r)} 0Z`;
}

/** A ring: two circles filled even-odd, for wheels. */
function ring(cx: number, cy: number, r: number, width: number): string {
  return circle(cx, cy, r) + circle(cx, cy, r - width);
}

/** A rounded segment from (x1, y1) to (x2, y2) with radius r: a limb between two joints. */
function capsule(x1: number, y1: number, x2: number, y2: number, r: number): string {
  const len = Math.hypot(x2 - x1, y2 - y1) || 1e-6;
  const nx = (-(y2 - y1) / len) * r;
  const ny = ((x2 - x1) / len) * r;
  return (
    `M${f(x1 + nx)} ${f(y1 + ny)}L${f(x2 + nx)} ${f(y2 + ny)}` +
    `A${f(r)} ${f(r)} 0 0 0 ${f(x2 - nx)} ${f(y2 - ny)}` +
    `L${f(x1 - nx)} ${f(y1 - ny)}A${f(r)} ${f(r)} 0 0 0 ${f(x1 + nx)} ${f(y1 + ny)}Z`
  );
}

function roundedRect(x: number, y: number, w: number, h: number, r: number): string {
  return (
    `M${f(x + r)} ${f(y)}H${f(x + w - r)}Q${f(x + w)} ${f(y)} ${f(x + w)} ${f(y + r)}` +
    `V${f(y + h - r)}Q${f(x + w)} ${f(y + h)} ${f(x + w - r)} ${f(y + h)}` +
    `H${f(x + r)}Q${f(x)} ${f(y + h)} ${f(x)} ${f(y + h - r)}V${f(y + r)}Q${f(x)} ${f(y)} ${f(x + r)} ${f(y)}Z`
  );
}

const body = (d: string): FigureShape => ({ d, tone: 'body' });
const gear = (d: string): FigureShape => ({ d, tone: 'gear' });
const frame = (d: string): FigureShape => ({ d, tone: 'frame' });

/**
 * A standing backpacker, front view, with a pack behind the shoulders and a pole in each
 * hand. Column by column: poles, arms, pack edge, four torso columns, pack edge, arms,
 * poles. That keeps each part in cells of its own, so bindings can separate them.
 */
const hiker: Figure = {
  id: 'hiker',
  label: 'Hiker',
  cols: 10,
  rows: 16,
  shapes: [
    gear(roundedRect(2.1, 1.6, 5.8, 7.0, 1.0)),
    frame(roundedRect(0.52, 8.9, 0.18, 6.8, 0.09)),
    frame(roundedRect(9.3, 8.9, 0.18, 6.8, 0.09)),
    body(capsule(2.35, 3.7, 1.35, 8.6, 0.5)),
    body(capsule(7.65, 3.7, 8.65, 8.6, 0.5)),
    body(circle(1.1, 9.35, 0.55)),
    body(circle(8.9, 9.35, 0.55)),
    body('M3.3 3.1H6.7Q7.35 3.1 7.3 3.8L6.9 8.5Q6.85 9 6.35 9H3.65Q3.15 9 3.1 8.5L2.7 3.8Q2.65 3.1 3.3 3.1Z'),
    body(roundedRect(4.5, 2.4, 1.0, 1.0, 0.3)),
    body(circle(5, 1.45, 1.15)),
    body(roundedRect(3.2, 8.5, 3.6, 1.5, 0.45)),
    body('M3.25 9.3H4.95L4.75 14.7H3.55Z'),
    body('M5.05 9.3H6.75L6.45 14.7H5.25Z'),
    body(roundedRect(2.7, 14.45, 2.25, 1.0, 0.45)),
    body(roundedRect(5.05, 14.45, 2.25, 1.0, 0.45)),
  ],
};

/**
 * A runner mid-stride, facing right. The front wrist sits alone in column 7, row 4, so a
 * watch can be bound to a single cell.
 */
const runner: Figure = {
  id: 'runner',
  label: 'Runner',
  cols: 10,
  rows: 16,
  shapes: [
    // Back limbs first, so the front ones overlap them.
    body(capsule(5.0, 3.6, 3.7, 5.7, 0.42)),
    body(capsule(3.7, 5.7, 2.7, 4.7, 0.36)),
    body(circle(2.55, 4.45, 0.42)),
    body(capsule(5.1, 9.0, 3.9, 11.3, 0.62)),
    body(capsule(3.9, 11.3, 1.9, 12.6, 0.48)),
    body(capsule(1.3, 12.35, 2.2, 13.35, 0.38)),
    // Torso, head and front limbs.
    body('M4.3 3.1Q5.6 2.55 6.75 3.15Q7.0 3.3 6.95 3.8L6.55 8.4Q6.4 9.2 5.4 9.2Q4.4 9.2 4.2 8.4L3.95 3.8Q3.9 3.3 4.3 3.1Z'),
    body(roundedRect(5.1, 2.3, 0.9, 1.0, 0.3)),
    body(circle(5.75, 1.4, 1.1)),
    body(capsule(5.9, 9.0, 7.3, 11.4, 0.66)),
    body(capsule(7.3, 11.4, 7.3, 14.2, 0.5)),
    body(roundedRect(6.8, 14.2, 2.4, 0.95, 0.45)),
    body(capsule(6.0, 3.6, 6.7, 5.9, 0.42)),
    body(capsule(6.7, 5.9, 7.6, 4.6, 0.36)),
    body(circle(7.95, 4.35, 0.42)),
  ],
};

/** A road cyclist on a bike, side view. */
const cyclist: Figure = {
  id: 'cyclist',
  label: 'Cyclist',
  cols: 16,
  rows: 12,
  shapes: [
    frame(ring(3.4, 8.5, 3.0, 0.38)),
    frame(ring(12.6, 8.5, 3.0, 0.38)),
    frame(capsule(3.4, 8.5, 7.2, 8.6, 0.14)),
    frame(capsule(7.2, 8.6, 6.0, 4.9, 0.15)),
    frame(capsule(6.0, 4.9, 11.2, 5.1, 0.15)),
    frame(capsule(7.2, 8.6, 11.3, 5.3, 0.16)),
    frame(capsule(11.2, 5.1, 12.6, 8.5, 0.15)),
    frame(capsule(3.4, 8.5, 6.0, 4.9, 0.12)),
    frame(capsule(5.3, 4.55, 6.7, 4.55, 0.2)),
    frame(capsule(11.1, 4.2, 11.8, 4.0, 0.16)),
    body(capsule(6.1, 4.2, 8.3, 5.9, 0.5)),
    body(capsule(8.3, 5.9, 7.5, 8.7, 0.38)),
    body(capsule(7.0, 9.0, 8.2, 9.0, 0.3)),
    body(capsule(6.1, 4.0, 9.9, 2.1, 0.8)),
    body(capsule(9.8, 2.2, 11.3, 4.0, 0.32)),
    body(circle(11.05, 1.35, 0.9)),
  ],
};

/** A pitched trekking-pole tent, side view. A prop: pitched, not worn. */
const tent: Figure = {
  id: 'tent',
  label: 'Tent',
  cols: 10,
  rows: 6,
  shapes: [
    gear('M0.4 5.7L4.3 0.55Q5 0.05 5.7 0.55L9.6 5.7Z'),
    frame('M3.7 5.7L5 2.4L6.3 5.7Z'),
    frame(roundedRect(0.2, 5.65, 9.6, 0.2, 0.1)),
  ],
};

export const FIGURES: Record<string, Figure> = { hiker, runner, cyclist, tent };

/** Fills for the resting mannequin: a grey form, with carried gear a shade darker. */
export const TONE_FILL: Record<Tone, string> = {
  body: 'url(#pdMannequin)',
  gear: '#57534e',
  frame: '#78716c',
};
