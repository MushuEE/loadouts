import type { ReactNode } from 'react';
import { AlertTriangle, Puzzle } from 'lucide-react';
import { api } from '../../api/client';
import type { PluginSurface, RenderedView } from '../../api/types';
import { useAsync } from '../../lib/useAsync';
import { Spinner } from '../ui';
import { PluginEmbed } from './PluginEmbed';
import { WidgetView } from './WidgetView';

interface PluginSurfaceProps {
  surface: PluginSurface;
  loadoutId?: string;
  itemId?: string;
  communityId?: string;
  /** Rendered when nothing is installed, so a surface can stay silent if it prefers. */
  emptyHint?: ReactNode;
}

/**
 * Renders every plugin view installed on one surface.
 *
 * The host asks the server once per surface and gets back a list of already-evaluated
 * views. Each renders in its own card, and a view that failed comes back carrying an
 * error rather than an absence — one broken plugin shows a message in its own box instead
 * of taking the page down with it.
 */
export function PluginSurfaceHost({ surface, loadoutId, itemId, communityId, emptyHint }: PluginSurfaceProps) {
  const { data, error, loading } = useAsync(
    () => api.renderSurface({ surface, loadout_id: loadoutId, item_id: itemId, community_id: communityId }),
    [surface, loadoutId, itemId, communityId],
  );

  if (loading) return <Spinner />;

  // A failure to reach the plugin service should not look like a broken page: the rest
  // of the loadout is still perfectly usable without its plugins.
  if (error) {
    return (
      <p className="text-xs text-stone-500 italic flex items-center gap-1.5">
        <AlertTriangle size={12} /> Plugins are unavailable right now.
      </p>
    );
  }

  const views = data?.views ?? [];
  if (views.length === 0) return <>{emptyHint ?? null}</>;

  const scope = scopeFor(surface, { loadoutId, itemId, communityId });

  return (
    <div className="space-y-3">
      {views.map((view) => (
        <PluginCard key={`${view.install_id}-${view.view_id}`} view={view} scope={scope} />
      ))}
    </div>
  );
}

function PluginCard({ view, scope }: { view: RenderedView; scope: { type: string; id: string } }) {
  return (
    <section className="bg-stone-900 border border-stone-800 rounded-xl overflow-hidden">
      <header className="flex items-center gap-2 px-4 py-2.5 border-b border-stone-800">
        <Puzzle size={14} className="text-orange-400 shrink-0" />
        <h3 className="text-sm font-semibold text-stone-200 flex-1 truncate">{view.title}</h3>
        <span className="text-[10px] uppercase tracking-wide text-stone-600">
          {view.kind === 'embed' ? 'sandboxed' : 'widget'} · v{view.version}
        </span>
      </header>

      <div className="p-4">
        {view.error ? (
          <p className="text-xs text-red-300 flex items-start gap-1.5">
            <AlertTriangle size={12} className="mt-0.5 shrink-0" />
            <span>
              This plugin could not render: {view.error}
            </span>
          </p>
        ) : view.kind === 'widget' && view.widget ? (
          <WidgetView widget={view.widget} />
        ) : view.kind === 'embed' ? (
          <PluginEmbed view={view} scope={scope} />
        ) : (
          <p className="text-xs text-stone-500 italic">Nothing to render.</p>
        )}
      </div>
    </section>
  );
}

/** The scope a plugin's storage writes are keyed to, which follows from the surface. */
function scopeFor(
  surface: PluginSurface,
  ids: { loadoutId?: string; itemId?: string; communityId?: string },
): { type: string; id: string } {
  switch (surface) {
    case 'loadout.panel':
    case 'loadout.sidebar':
      return { type: 'loadout', id: ids.loadoutId ?? '' };
    case 'item.tab':
      return { type: 'item', id: ids.itemId ?? '' };
    case 'community.tab':
      return { type: 'community', id: ids.communityId ?? '' };
  }
}
