import { AlertTriangle, Loader2 } from 'lucide-react';
import type { LoadoutSummary } from '../api/types';
import { formatCost, formatKg } from '../lib/display';

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 text-stone-500 text-sm p-8">
      <Loader2 className="w-4 h-4 animate-spin" />
      {label}
    </div>
  );
}

export function ErrorNote({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-3 p-4 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
      <div>{message}</div>
    </div>
  );
}

export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="text-center py-16 border border-dashed border-stone-800 rounded-xl">
      <div className="text-stone-400 font-medium">{title}</div>
      {hint && <div className="text-stone-600 text-sm mt-1">{hint}</div>}
    </div>
  );
}

export function Badge({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${className}`}>
      {children}
    </span>
  );
}

/** A loadout card used in Home, Discover, and community feeds. */
export function LoadoutCard({
  summary,
  onOpen,
  onFork,
  showOwner = true,
}: {
  summary: LoadoutSummary;
  onOpen: () => void;
  onFork?: () => void;
  showOwner?: boolean;
}) {
  const { loadout, stats } = summary;
  return (
    <div className="bg-stone-900 border border-stone-800 rounded-xl p-5 hover:border-orange-500/60 transition-colors flex flex-col">
      <div className="flex items-start justify-between gap-3">
        <button onClick={onOpen} className="text-left">
          <h3 className="font-bold text-stone-100 leading-tight">{loadout.name}</h3>
          {showOwner && (
            <div className="text-xs text-stone-500 mt-1">
              @{summary.owner_handle} · {summary.template_name}
            </div>
          )}
        </button>
        <div className="flex gap-1 shrink-0">
          {loadout.status === 'draft' && <Badge className="bg-stone-800 text-stone-400">Draft</Badge>}
          {loadout.visibility === 'public' && <Badge className="bg-emerald-500/15 text-emerald-400">Public</Badge>}
          {loadout.visibility === 'private' && <Badge className="bg-stone-800 text-stone-500">Private</Badge>}
        </div>
      </div>

      {loadout.description && (
        <p className="text-sm text-stone-500 mt-3 line-clamp-2">{loadout.description}</p>
      )}

      <div className="mt-4 flex gap-4 text-xs font-mono text-stone-400">
        <span title="Base weight excludes consumables">base {formatKg(stats.base_weight_g)}</span>
        <span>total {formatKg(stats.total_weight_g)}</span>
        <span>{formatCost(stats.total_cost_cents)}</span>
      </div>

      {summary.item_preview.length > 0 && (
        <div className="mt-3 text-[11px] text-stone-600 truncate">{summary.item_preview.join(' · ')}</div>
      )}

      <div className="mt-4 pt-4 border-t border-stone-800/70 flex gap-2">
        <button
          onClick={onOpen}
          className="px-3 py-1.5 rounded-lg bg-stone-800 hover:bg-stone-700 text-stone-200 text-xs font-medium"
        >
          Open
        </button>
        {onFork && (
          <button
            onClick={onFork}
            className="px-3 py-1.5 rounded-lg bg-orange-500/15 hover:bg-orange-500/25 text-orange-300 text-xs font-medium"
          >
            Fork {loadout.fork_count > 0 && `(${loadout.fork_count})`}
          </button>
        )}
      </div>
    </div>
  );
}
