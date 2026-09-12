import type { RenderedPoint, WidgetRender } from '../../api/types';

/**
 * Draws an evaluated widget.
 *
 * Everything here is a dumb renderer by design. The server evaluated the plugin's
 * expressions and produced literal values and display strings, so this component never
 * sees anything an author wrote and cannot be made to execute it. Formatting already
 * happened server-side too, which is why cells print `display` rather than reformatting
 * `value` — two surfaces showing the same widget should not disagree about decimals.
 *
 * The charts are hand-rolled SVG. A charting library would be the obvious reach here, but
 * the app has no runtime dependencies beyond React and these shapes are simple.
 */
export function WidgetView({ widget }: { widget: WidgetRender }) {
  if (widget.empty) {
    return (
      <div className="text-sm text-stone-500 italic py-6 text-center">
        {widget.empty_text || 'Nothing to show yet.'}
      </div>
    );
  }

  return (
    <div>
      {widget.type === 'stat_grid' && <StatGrid widget={widget} />}
      {widget.type === 'bar_chart' && <BarChart widget={widget} />}
      {widget.type === 'pie_chart' && <PieChart widget={widget} />}
      {widget.type === 'table' && <DataTable widget={widget} />}
      {widget.truncated && (
        <p className="text-[11px] text-stone-500 mt-2 italic">Showing the top results only.</p>
      )}
    </div>
  );
}

function StatGrid({ widget }: { widget: WidgetRender }) {
  const stats = widget.stats ?? [];
  return (
    <div className="grid grid-cols-2 lg:grid-cols-3 gap-2">
      {stats.map((stat) => (
        <div key={stat.label} className="bg-stone-900/60 rounded-lg p-3" title={stat.help}>
          <div className="text-[11px] uppercase tracking-wide text-stone-500">{stat.label}</div>
          <div className="text-lg font-semibold text-stone-100 tabular-nums">
            {stat.display}
            {stat.unit && <span className="text-xs text-stone-400 ml-1">{stat.unit}</span>}
          </div>
        </div>
      ))}
    </div>
  );
}

function DataTable({ widget }: { widget: WidgetRender }) {
  const columns = widget.columns ?? [];
  const rows = widget.rows ?? [];
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-[11px] uppercase tracking-wide text-stone-500 border-b border-stone-800">
            {columns.map((col, i) => (
              <th key={i} className="py-2 px-2 font-medium" style={{ textAlign: align(col.align) }}>
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, r) => (
            <tr key={r} className="border-b border-stone-800/50 hover:bg-stone-800/30">
              {row.map((cell, c) => (
                <td
                  key={c}
                  className="py-1.5 px-2 text-stone-300 tabular-nums"
                  style={{ textAlign: align(columns[c]?.align) }}
                >
                  {cell.display}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BarChart({ widget }: { widget: WidgetRender }) {
  const points = widget.points ?? [];
  const max = Math.max(...points.map((p) => p.value), 0) || 1;

  return (
    <div className="space-y-1.5">
      {points.map((point, i) => (
        <div key={`${point.label}-${i}`} className="flex items-center gap-2">
          <div className="w-28 shrink-0 text-xs text-stone-400 truncate" title={point.label}>
            {point.label}
          </div>
          <div className="flex-1 h-5 bg-stone-900 rounded overflow-hidden">
            <div
              className="h-full rounded transition-all"
              style={{ width: `${(point.value / max) * 100}%`, background: colorFor(i) }}
            />
          </div>
          <div className="w-20 shrink-0 text-right text-xs text-stone-300 tabular-nums">
            {point.display}
          </div>
        </div>
      ))}
    </div>
  );
}

function PieChart({ widget }: { widget: WidgetRender }) {
  const points = widget.points ?? [];

  return (
    <div className="flex items-center gap-5 flex-wrap">
      <svg viewBox="-1.1 -1.1 2.2 2.2" className="w-36 h-36 shrink-0 -rotate-90">
        {pieSlices(points).map((slice, i) => (
          <path key={i} d={slice} fill={colorFor(i)} stroke="#1c1917" strokeWidth={0.012} />
        ))}
        {/* A donut hole keeps the slices readable at this size. */}
        <circle cx={0} cy={0} r={0.55} fill="#1c1917" />
      </svg>

      <ul className="flex-1 min-w-[180px] space-y-1">
        {points.map((point, i) => (
          <li key={`${point.label}-${i}`} className="flex items-center gap-2 text-xs">
            <span className="w-2.5 h-2.5 rounded-sm shrink-0" style={{ background: colorFor(i) }} />
            <span className="text-stone-300 truncate flex-1" title={point.label}>
              {point.label}
            </span>
            <span className="text-stone-400 tabular-nums">{point.display}</span>
            <span className="text-stone-600 tabular-nums w-10 text-right">
              {(point.share * 100).toFixed(0)}%
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Builds pie slice paths on a unit circle.
 *
 * Shares come from the server precomputed, so the geometry here agrees with the
 * percentages printed in the legend instead of rounding independently.
 */
function pieSlices(points: RenderedPoint[]): string[] {
  const paths: string[] = [];
  let start = 0;

  for (const point of points) {
    const share = Number.isFinite(point.share) ? point.share : 0;
    if (share <= 0) continue;

    // A single slice covering everything cannot be drawn as an arc (start and end land
    // on the same point), so draw it as two half circles.
    if (share >= 0.9999) {
      paths.push('M 1 0 A 1 1 0 0 1 -1 0 A 1 1 0 0 1 1 0 Z');
      continue;
    }

    const end = start + share * Math.PI * 2;
    const [x1, y1] = [Math.cos(start), Math.sin(start)];
    const [x2, y2] = [Math.cos(end), Math.sin(end)];
    const largeArc = share > 0.5 ? 1 : 0;
    paths.push(`M 0 0 L ${x1} ${y1} A 1 1 0 ${largeArc} 1 ${x2} ${y2} Z`);
    start = end;
  }
  return paths;
}

// A fixed palette keyed by position, so the same chart colours the same way on every load.
const palette = ['#f97316', '#f59e0b', '#84cc16', '#14b8a6', '#38bdf8', '#a78bfa', '#f472b6', '#fb7185'];

function colorFor(index: number): string {
  return palette[index % palette.length];
}

function align(value?: string): 'left' | 'right' | 'center' {
  if (value === 'right' || value === 'center') return value;
  return 'left';
}
