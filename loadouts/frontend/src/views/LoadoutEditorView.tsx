import { useMemo, useState } from 'react';
import { ArrowUpLeft, ChevronRight, GitFork, Globe, Lock, Maximize2, Plus, Trash2 } from 'lucide-react';
import { api } from '../api/client';
import type { LoadoutDetail, LoadoutEntry, ResolvedEntry, SlotDefinition, Visibility } from '../api/types';
import { useSession } from '../session/SessionContext';
import { useAsync } from '../lib/useAsync';
import { categoryIcon, formatCost, formatGrams, formatKg } from '../lib/display';
import { Badge, ErrorNote, Spinner } from '../components/ui';
import { FavoritePanel } from '../components/FavoritePanel';
import { ItemPickerModal } from '../components/ItemPickerModal';
import { PluginSurfaceHost } from '../components/plugins/PluginSurfaceHost';
import { Paperdoll } from '../components/Paperdoll';
import { isMapped, targetForSlot } from '../paperdoll/archetypes';

/** Flatten the server's nested entry tree back into the flat list the API expects on write. */
function flatten(entries: ResolvedEntry[]): LoadoutEntry[] {
  const out: LoadoutEntry[] = [];
  const walk = (nodes: ResolvedEntry[]) => {
    nodes.forEach((node) => {
      out.push(node.entry);
      if (node.children) walk(node.children);
    });
  };
  walk(entries);
  return out;
}

function findNode(entries: ResolvedEntry[], entryId: string): ResolvedEntry | null {
  for (const node of entries) {
    if (node.entry.id === entryId) return node;
    const found = node.children ? findNode(node.children, entryId) : null;
    if (found) return found;
  }
  return null;
}

/** Collect an entry and all of its descendants, so removing a container removes its contents. */
function subtreeIds(node: ResolvedEntry): string[] {
  const ids = [node.entry.id];
  node.children?.forEach((child) => ids.push(...subtreeIds(child)));
  return ids;
}

export function LoadoutEditorView({
  loadoutId,
  onBack,
  onOpenLoadout,
}: {
  loadoutId: string;
  onBack: () => void;
  /** Navigate to another loadout. Forking produces a new one, and without this the editor
   *  has no way to take you there. */
  onOpenLoadout: (id: string) => void;
}) {
  const { profile } = useSession();
  const detail = useAsync<LoadoutDetail>(() => api.getLoadout(loadoutId), [loadoutId]);
  // Path of entry IDs we have zoomed into (pack -> pocket -> ditty bag).
  const [path, setPath] = useState<string[]>([]);
  const [picking, setPicking] = useState<{ slot: SlotDefinition; parentEntryId: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const data = detail.data;
  const isOwner = !!profile && data?.loadout.owner_profile_id === profile.id;

  // The node we're currently inside; null means we're at the template root.
  const currentNode = useMemo(() => {
    if (!data || path.length === 0) return null;
    return findNode(data.entries, path[path.length - 1]);
  }, [data, path]);

  const currentEntries: ResolvedEntry[] = currentNode ? currentNode.children ?? [] : data?.entries ?? [];
  const slots: SlotDefinition[] = currentNode
    ? currentNode.item.provided_slots ?? []
    : data?.template.version.slots ?? [];

  // Slots the paperdoll cannot place on a figure stay in the grid. Inside a container the
  // paperdoll is not shown at all, so the grid takes everything.
  const gridSlots = useMemo(
    () => (path.length === 0 ? slots.filter((s) => !isMapped(targetForSlot(s))) : slots),
    [slots, path.length],
  );

  async function persist(entries: LoadoutEntry[]) {
    setBusy(true);
    setError(null);
    try {
      await api.replaceEntries(loadoutId, entries);
      detail.reload();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function addItem(itemId: string, slot: SlotDefinition, parentEntryId: string) {
    if (!data) return;
    const all = flatten(data.entries);
    const siblings = all.filter((e) => e.parent_entry_id === parentEntryId);
    all.push({
      id: crypto.randomUUID(),
      slot_id: slot.id,
      parent_entry_id: parentEntryId,
      item_id: itemId,
      quantity: 1,
      note: '',
      position: siblings.length,
    });
    persist(all);
  }

  function removeEntry(node: ResolvedEntry) {
    if (!data) return;
    const doomed = new Set(subtreeIds(node));
    persist(flatten(data.entries).filter((e) => !doomed.has(e.id)));
  }

  async function publish(visibility: Visibility) {
    setBusy(true);
    setError(null);
    try {
      await api.publishLoadout(loadoutId, { visibility });
      detail.reload();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function fork() {
    try {
      const forked = await api.forkLoadout(loadoutId);
      setPath([]);
      // Navigate to the copy. This used to set window.location.hash, which nothing in the
      // app reads, and then reload the *original* - so forking looked like a no-op.
      onOpenLoadout(forked.loadout.id);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  if (detail.loading) return <Spinner label="Loading loadout…" />;
  if (detail.error) return <div className="p-8 flex-1"><ErrorNote message={detail.error} /></div>;
  if (!data) return null;

  // Freeform templates define no slots, so we synthesize one open slot per existing entry
  // plus a trailing empty one; structured templates render their fixed slot grid.
  const freeform = slots.length === 0;
  const nextFreeSlot: SlotDefinition = {
    id: `free-${Date.now().toString(36)}`,
    name: 'Add item',
    accepted_categories: ['universal'],
    required: false,
    max_items: 1,
    position: currentEntries.length,
  };

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* Stats rail */}
      <aside className="w-72 bg-stone-900 border-r border-stone-800 flex flex-col shrink-0 overflow-y-auto">
        <div className="p-6 border-b border-stone-800">
          <button onClick={onBack} className="text-xs text-stone-500 hover:text-white mb-3">
            ← All loadouts
          </button>
          <h1 className="text-lg font-bold text-stone-100 leading-tight">{data.loadout.name}</h1>
          <div className="text-[11px] text-stone-500 mt-1">
            @{data.owner.handle} · {data.template.template.name} v{data.loadout.template_version}
          </div>
        </div>

        <div className="p-6 space-y-4">
          <div className="p-4 rounded-xl bg-orange-500/10 border border-orange-500/20">
            <div className="text-[10px] font-bold text-orange-400 uppercase tracking-widest mb-1">Base Weight</div>
            <div className="text-3xl font-light text-white">{formatKg(data.stats.base_weight_g)}</div>
            <div className="text-[11px] text-orange-300/60 mt-1">Excludes consumables</div>
          </div>
          <div className="p-4 rounded-xl bg-stone-800 border border-stone-700">
            <div className="text-[10px] font-bold text-stone-500 uppercase tracking-widest mb-1">Total Weight</div>
            <div className="text-2xl font-light text-white">{formatKg(data.stats.total_weight_g)}</div>
            <div className="text-[11px] text-stone-500 mt-2">
              Consumables {formatKg(data.stats.consumable_weight_g)}
            </div>
          </div>
          <div className="p-4 rounded-xl bg-stone-800/60 border border-stone-800">
            <div className="text-[10px] font-bold text-stone-500 uppercase tracking-widest mb-1">Cost</div>
            <div className="text-xl font-light text-white">{formatCost(data.stats.total_cost_cents)}</div>
            <div className="text-[11px] text-stone-500 mt-2">{data.stats.item_count} items</div>
          </div>
        </div>

        {data.issues.length > 0 && (
          <div className="px-6 pb-6 space-y-2">
            <div className="text-[10px] font-bold uppercase tracking-widest text-stone-500">Validation</div>
            {data.issues.map((issue, i) => (
              <div
                key={i}
                className={`text-[11px] p-2 rounded-lg border ${
                  issue.severity === 'error'
                    ? 'bg-red-500/10 border-red-500/30 text-red-300'
                    : 'bg-amber-500/10 border-amber-500/30 text-amber-300'
                }`}
              >
                {issue.message}
              </div>
            ))}
          </div>
        )}

        {/* Endorsements sit with the stats because they are a judgement about this
            loadout, not an action on it. */}
        <FavoritePanel loadoutId={loadoutId} />

        {/* Sidebar plugins sit with the stats, since that is what they annotate. */}
        <div className="px-4 pb-4">
          <PluginSurfaceHost surface="loadout.sidebar" loadoutId={loadoutId} />
        </div>

        <div className="mt-auto p-6 border-t border-stone-800 space-y-2">
          {isOwner ? (
            <>
              <div className="flex items-center gap-2 text-[11px] text-stone-500">
                <Badge className="bg-stone-800 text-stone-400">{data.loadout.status}</Badge>
                <Badge className="bg-stone-800 text-stone-400">{data.loadout.visibility}</Badge>
              </div>
              <button
                onClick={() => publish('public')}
                disabled={busy}
                className="w-full flex items-center justify-center gap-2 px-4 py-2 rounded-lg bg-orange-500 hover:bg-orange-400 text-sm font-medium disabled:opacity-50"
              >
                <Globe className="w-4 h-4" /> Publish publicly
              </button>
              <button
                onClick={() => publish('private')}
                disabled={busy}
                className="w-full px-4 py-2 rounded-lg bg-stone-800 hover:bg-stone-700 text-xs text-stone-300 disabled:opacity-50"
              >
                Make private
              </button>
            </>
          ) : (
            <button
              onClick={fork}
              className="w-full flex items-center justify-center gap-2 px-4 py-2 rounded-lg bg-orange-500/15 hover:bg-orange-500/25 text-orange-300 text-sm font-medium"
            >
              <GitFork className="w-4 h-4" /> Fork this loadout
            </button>
          )}
        </div>
      </aside>

      {/* Slot grid */}
      <main className="flex-1 flex flex-col bg-stone-950 overflow-hidden">
        <header className="h-16 border-b border-stone-800 flex items-center px-8 bg-stone-900/50 shrink-0">
          <button
            onClick={() => setPath([])}
            className={`text-sm ${path.length === 0 ? 'text-white' : 'text-stone-500 hover:text-white'}`}
          >
            {data.loadout.name}
          </button>
          {path.map((entryId, idx) => {
            const node = findNode(data.entries, entryId);
            return (
              <span key={entryId} className="flex items-center">
                <ChevronRight className="w-4 h-4 text-stone-600 mx-2" />
                <button
                  onClick={() => setPath(path.slice(0, idx + 1))}
                  className={`text-sm ${idx === path.length - 1 ? 'text-white' : 'text-stone-500 hover:text-white'}`}
                >
                  {node?.item.name ?? 'Container'}
                </button>
              </span>
            );
          })}
          {path.length > 0 && (
            <button
              onClick={() => setPath(path.slice(0, -1))}
              className="ml-auto flex items-center gap-1.5 text-xs text-stone-400 hover:text-white"
            >
              <ArrowUpLeft className="w-3.5 h-3.5" /> Up
            </button>
          )}
        </header>

        <div className="flex-1 overflow-y-auto p-8">
          <div className="max-w-4xl mx-auto">
            {error && <div className="mb-4"><ErrorNote message={error} /></div>}

            {/* Editing is owner-only, and the add buttons simply vanish for everyone else.
                Without this the loadout reads as broken rather than as someone else's. */}
            {!isOwner && (
              <div className="mb-6 flex items-center gap-4 p-4 rounded-xl bg-stone-900 border border-stone-800">
                <Lock className="w-4 h-4 text-stone-500 shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-stone-300">
                    Read-only — this loadout belongs to @{data.owner.handle}
                  </div>
                  <div className="text-[11px] text-stone-500 mt-0.5">
                    {profile ? <>You are acting as @{profile.handle}. </> : null}
                    Fork it to get an editable copy of your own.
                  </div>
                </div>
                <button
                  onClick={fork}
                  className="shrink-0 flex items-center gap-2 px-3 py-2 rounded-lg bg-orange-500/15 hover:bg-orange-500/25 text-orange-300 text-xs font-medium"
                >
                  <GitFork className="w-3.5 h-3.5" /> Fork
                </button>
              </div>
            )}

            {data.loadout.description && path.length === 0 && (
              <p className="text-stone-500 text-sm mb-6">{data.loadout.description}</p>
            )}

            {/* The paperdoll takes the slots it can place on a figure; the grid keeps the
                rest. Only at the template root - inside a container the slots are the
                container's own compartments, which are not body parts. */}
            {path.length === 0 && (
              <Paperdoll
                templateName={data.template.template.name}
                slots={slots}
                entries={currentEntries}
                readOnly={!isOwner}
                onPick={(slot) => setPicking({ slot, parentEntryId: currentNode?.entry.id ?? '' })}
                onRemove={removeEntry}
              />
            )}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {/* Structured slots from the template (or the container item). */}
              {gridSlots.map((slot) => {
                const occupants = currentEntries.filter((e) => e.entry.slot_id === slot.id);
                return (
                  <SlotCell
                    key={slot.id}
                    slot={slot}
                    occupants={occupants}
                    readOnly={!isOwner}
                    onPick={() => setPicking({ slot, parentEntryId: currentNode?.entry.id ?? '' })}
                    onZoom={(node) => setPath([...path, node.entry.id])}
                    onRemove={removeEntry}
                  />
                );
              })}

              {/* Freeform level: existing items plus one open slot. */}
              {freeform &&
                currentEntries.map((node) => (
                  <SlotCell
                    key={node.entry.id}
                    slot={{
                      id: node.entry.slot_id,
                      name: 'Item',
                      accepted_categories: ['universal'],
                      required: false,
                      max_items: 1,
                      position: node.entry.position,
                    }}
                    occupants={[node]}
                    readOnly={!isOwner}
                    onPick={() => setPicking({ slot: nextFreeSlot, parentEntryId: currentNode?.entry.id ?? '' })}
                    onZoom={(n) => setPath([...path, n.entry.id])}
                    onRemove={removeEntry}
                  />
                ))}

              {freeform && isOwner && (
                <button
                  onClick={() => setPicking({ slot: nextFreeSlot, parentEntryId: currentNode?.entry.id ?? '' })}
                  className="aspect-[4/3] rounded-xl border border-dashed border-stone-800 hover:border-orange-500/60 text-stone-600 hover:text-orange-400 flex flex-col items-center justify-center gap-2"
                >
                  <Plus className="w-6 h-6" />
                  <span className="text-[11px] uppercase font-bold tracking-wider">Add item</span>
                </button>
              )}
            </div>

            {slots.length === 0 && currentEntries.length === 0 && !isOwner && (
              <div className="text-stone-600 text-sm py-12 text-center">This container is empty.</div>
            )}

            {/* Plugin panels. Only at the top level: a plugin reasons about the whole
                loadout, so showing it while zoomed into a container would be a lie. */}
            {path.length === 0 && (
              <div className="mt-8">
                <PluginSurfaceHost surface="loadout.panel" loadoutId={loadoutId} />
              </div>
            )}
          </div>
        </div>
      </main>

      {picking && (
        <ItemPickerModal
          slot={picking.slot}
          onClose={() => setPicking(null)}
          onSelect={(itemId) => {
            addItem(itemId, picking.slot, picking.parentEntryId);
            setPicking(null);
          }}
        />
      )}
    </div>
  );
}

function SlotCell({
  slot,
  occupants,
  readOnly,
  onPick,
  onZoom,
  onRemove,
}: {
  slot: SlotDefinition;
  occupants: ResolvedEntry[];
  readOnly: boolean;
  onPick: () => void;
  onZoom: (node: ResolvedEntry) => void;
  onRemove: (node: ResolvedEntry) => void;
}) {
  const empty = occupants.length === 0;
  // max_items 0 means "exactly one" and -1 means unlimited. See core.SlotDefinition.
  const capacity = slot.max_items === -1 ? Infinity : Math.max(slot.max_items, 1);
  const full = occupants.length >= capacity;

  return (
    <div
      className={`rounded-xl border overflow-hidden flex flex-col min-h-[9rem] ${
        empty ? 'bg-stone-900/40 border-dashed border-stone-800' : 'bg-stone-900 border-stone-700'
      }`}
    >
      <div className="flex items-center justify-between px-3 py-2 bg-black/20">
        <span className="text-[10px] font-bold text-stone-500 uppercase tracking-widest truncate">
          {slot.name}
          {slot.required && <span className="text-orange-500/80"> *</span>}
        </span>
        {/* Three distinct states. Previously "full" and "read-only" both rendered as a
            bare absence of the + button, which reads as a broken slot either way. */}
        {!readOnly && !full && (
          <button onClick={onPick} className="text-stone-600 hover:text-orange-400" title="Add item">
            <Plus className="w-3.5 h-3.5" />
          </button>
        )}
        {!readOnly && full && capacity !== Infinity && (
          <span
            className="text-[9px] font-medium text-stone-600 tabular-nums shrink-0"
            title={`This slot holds ${capacity === 1 ? 'one item' : `${capacity} items`}. Remove one to swap.`}
          >
            {occupants.length}/{capacity}
          </span>
        )}
      </div>

      <div className="flex-1 p-3 space-y-2">
        {empty ? (
          <button
            onClick={readOnly ? undefined : onPick}
            className="w-full h-full min-h-[4rem] flex flex-col items-center justify-center gap-1 text-stone-700 hover:text-orange-400 disabled:hover:text-stone-700"
            disabled={readOnly}
          >
            <span className="opacity-40">{categoryIcon(slot.accepted_categories[0] ?? 'universal', 'w-6 h-6')}</span>
            <span className="text-[10px] uppercase font-bold tracking-wider">Empty</span>
          </button>
        ) : (
          occupants.map((node) => {
            const hasSlots = (node.item.provided_slots ?? []).length > 0;
            const childCount = node.children?.length ?? 0;
            return (
              <div key={node.entry.id} className="group">
                <div className="flex items-start gap-2">
                  <span className="text-stone-400 mt-0.5">{categoryIcon(node.item.category, 'w-4 h-4')}</span>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-stone-200 leading-tight">
                      {node.item.name}
                      {node.entry.quantity > 1 && (
                        <span className="text-stone-500 font-mono text-xs"> ×{node.entry.quantity}</span>
                      )}
                    </div>
                    <div className="flex gap-2 text-[10px] font-mono text-stone-500 mt-1">
                      <span>{formatGrams(Number(node.item.metadata?.core?.weight_g ?? 0) * node.entry.quantity)}</span>
                      <span>{formatCost(Number(node.item.metadata?.core?.cost_cents ?? 0) * node.entry.quantity)}</span>
                    </div>
                  </div>
                  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    {hasSlots && (
                      <button onClick={() => onZoom(node)} className="text-stone-500 hover:text-white" title="Open container">
                        <Maximize2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                    {!readOnly && (
                      <button onClick={() => onRemove(node)} className="text-stone-600 hover:text-red-400" title="Remove">
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                </div>
                {hasSlots && (
                  <button
                    onClick={() => onZoom(node)}
                    className="mt-2 text-[10px] text-stone-600 hover:text-orange-400 uppercase tracking-wider"
                  >
                    {childCount} inside →
                  </button>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
