import { useMemo, useState } from 'react';
import {
  ArrowUpLeft,
  ChevronRight,
  GitFork,
  Globe,
  Image as ImageIcon,
  Lock,
  Maximize2,
  Plus,
  PlusSquare,
  Trash2,
} from 'lucide-react';
import { api } from '../api/client';
import type { LoadoutDetail, LoadoutEntry, ResolvedEntry, SlotDefinition, Visibility } from '../api/types';
import { useSession } from '../session/SessionContext';
import { useAsync } from '../lib/useAsync';
import { categoryIcon, formatCost, formatGrams, formatKg } from '../lib/display';
import { ItemThumb } from '../components/ItemThumb';
import { Badge, ErrorNote, Spinner } from '../components/ui';
import { FavoritePanel } from '../components/FavoritePanel';
import { ItemPickerModal } from '../components/ItemPickerModal';
import { ItemDetailPanel } from '../components/ItemDetailPanel';
import { PluginSurfaceHost } from '../components/plugins/PluginSurfaceHost';
import { Paperdoll } from '../components/Paperdoll';
import { CoverImage, CoverPicker } from '../components/CoverPicker';
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
  // The entry whose details are pinned open in the right rail. An id rather than the node,
  // so a reload after an edit re-resolves it instead of showing a stale copy.
  const [selectedEntryId, setSelectedEntryId] = useState<string | null>(null);
  const [addingSlot, setAddingSlot] = useState(false);
  const [editingCover, setEditingCover] = useState(false);

  const data = detail.data;
  const isOwner = !!profile && data?.loadout.owner_profile_id === profile.id;

  // The node we're currently inside; null means we're at the template root.
  const currentNode = useMemo(() => {
    if (!data || path.length === 0) return null;
    return findNode(data.entries, path[path.length - 1]);
  }, [data, path]);

  const selectedNode = useMemo(
    () => (data && selectedEntryId ? findNode(data.entries, selectedEntryId) : null),
    [data, selectedEntryId],
  );

  const currentEntries: ResolvedEntry[] = currentNode ? currentNode.children ?? [] : data?.entries ?? [];

  // Slots the owner added live on the loadout, not on the template. The server merges them
  // in EffectiveSlots; the client has to do the same or a custom slot would be accepted on
  // write and then never rendered.
  const extraSlots = data?.loadout.extra_slots ?? [];
  const slots: SlotDefinition[] = currentNode
    ? currentNode.item.provided_slots ?? []
    : [...(data?.template.version.slots ?? []), ...extraSlots];

  // Custom slots are the only removable ones, so the grid needs to be able to tell them
  // apart from the template's.
  const customSlotIds = useMemo(() => new Set(extraSlots.map((s) => s.id)), [extraSlots]);

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
    // The inspector is pinned to an id, so removing the thing it points at would leave a
    // panel describing gear that is no longer here.
    if (selectedEntryId && doomed.has(selectedEntryId)) setSelectedEntryId(null);
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

  async function setCover(url: string) {
    setBusy(true);
    setError(null);
    try {
      await api.updateLoadout(loadoutId, { cover_image_url: url });
      setEditingCover(false);
      detail.reload();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  /**
   * Define a slot on this loadout alone. The template is untouched, so this does not change
   * what anyone else's copy looks like - it is the cheap version of "I need somewhere to put
   * my dog's booties" that does not require owning the template.
   */
  async function addSlot(name: string, category: string) {
    setBusy(true);
    setError(null);
    try {
      await api.addLoadoutSlot(loadoutId, {
        name,
        accepted_categories: category ? [category] : ['universal'],
        required: false,
        max_items: 1,
      });
      setAddingSlot(false);
      detail.reload();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function removeSlot(slot: SlotDefinition) {
    const occupied = currentEntries.filter((e) => e.entry.slot_id === slot.id).length;
    // The server takes the contents with the slot rather than orphaning them. That is the
    // right behaviour but a surprising one, so say it out loud before doing it.
    const warning = occupied
      ? `Remove "${slot.name}"? The ${occupied === 1 ? 'item' : `${occupied} items`} in it will be removed too.`
      : `Remove "${slot.name}"?`;
    if (!window.confirm(warning)) return;
    setBusy(true);
    setError(null);
    try {
      await api.removeLoadoutSlot(loadoutId, slot.id);
      detail.reload();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (detail.loading) return <Spinner label="Loading loadout…" />;
  if (detail.error) return <div className="p-8 flex-1"><ErrorNote message={detail.error} /></div>;
  if (!data) return null;

  // Freeform templates define no slots, so we synthesize one open slot per loose entry plus
  // a trailing empty one; structured templates render their fixed slot grid.
  //
  // This asks the *template*, not the merged list. Adding a single custom slot to a Freeform
  // loadout would otherwise flip this flag and strand every existing item: they sit in
  // synthesised `free-*` slots that the grid only renders on the freeform path.
  const declaredSlots = currentNode ? currentNode.item.provided_slots ?? [] : data.template.version.slots ?? [];
  const freeform = declaredSlots.length === 0;

  // Entries that belong to no slot in the effective list. On a freeform loadout that is
  // everything; elsewhere it catches items left behind by a template version bump.
  const slotIds = new Set(slots.map((s) => s.id));
  const looseEntries = currentEntries.filter((e) => !slotIds.has(e.entry.slot_id));

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
        <CoverImage url={data.loadout.cover_image_url} className="h-32 shrink-0" />
        <div className="p-6 border-b border-stone-800">
          <button onClick={onBack} className="text-xs text-stone-500 hover:text-white mb-3">
            ← All loadouts
          </button>
          <h1 className="text-lg font-bold text-stone-100 leading-tight">{data.loadout.name}</h1>
          <div className="text-[11px] text-stone-500 mt-1">
            @{data.owner.handle} · {data.template.template.name} v{data.loadout.template_version}
          </div>
          {isOwner && (
            <button
              onClick={() => setEditingCover(!editingCover)}
              className="mt-2 text-[11px] text-stone-500 hover:text-orange-400 flex items-center gap-1"
            >
              <ImageIcon className="w-3 h-3" />
              {editingCover ? 'Done' : data.loadout.cover_image_url ? 'Change cover' : 'Add a cover'}
            </button>
          )}
          {editingCover && (
            <div className="mt-3">
              <CoverPicker value={data.loadout.cover_image_url} onChange={setCover} columns="grid-cols-3" />
            </div>
          )}
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
                selectedEntryId={selectedEntryId}
                onPick={(slot) => setPicking({ slot, parentEntryId: currentNode?.entry.id ?? '' })}
                onRemove={removeEntry}
                onSelect={setSelectedEntryId}
              />
            )}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {/* Structured slots from the template (or the container item). */}
              {gridSlots.map((slot) => {
                const occupants = currentEntries.filter((e) => e.entry.slot_id === slot.id);
                const custom = customSlotIds.has(slot.id);
                return (
                  <SlotCell
                    key={slot.id}
                    slot={slot}
                    occupants={occupants}
                    readOnly={!isOwner}
                    custom={custom}
                    selectedEntryId={selectedEntryId}
                    onPick={() => setPicking({ slot, parentEntryId: currentNode?.entry.id ?? '' })}
                    onZoom={(node) => setPath([...path, node.entry.id])}
                    onRemove={removeEntry}
                    onSelect={setSelectedEntryId}
                    // Only a slot you invented can be taken away. A template slot belongs to
                    // the version this loadout is pinned to, not to you.
                    onRemoveSlot={isOwner && custom ? () => removeSlot(slot) : undefined}
                  />
                );
              })}

              {/* Loose items: everything not claimed by a slot, one synthesised cell each. */}
              {looseEntries.map((node) => (
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
                  selectedEntryId={selectedEntryId}
                  onPick={() => setPicking({ slot: nextFreeSlot, parentEntryId: currentNode?.entry.id ?? '' })}
                  onZoom={(n) => setPath([...path, n.entry.id])}
                  onRemove={removeEntry}
                  onSelect={setSelectedEntryId}
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

              {/* Inventing a slot is only offered at the root. Inside a container the slots
                  are the container's own compartments, which come from the item, not from
                  you - a pocket you made up would have nowhere to live. */}
              {isOwner && path.length === 0 && !addingSlot && (
                <button
                  onClick={() => setAddingSlot(true)}
                  className="aspect-[4/3] rounded-xl border border-dashed border-stone-800 hover:border-sky-500/60 text-stone-600 hover:text-sky-400 flex flex-col items-center justify-center gap-2"
                >
                  <PlusSquare className="w-6 h-6" />
                  <span className="text-[11px] uppercase font-bold tracking-wider">Add slot</span>
                  <span className="text-[10px] text-stone-700 px-3 text-center leading-snug">
                    Yours only — the template is untouched
                  </span>
                </button>
              )}

              {addingSlot && (
                <AddSlotForm busy={busy} onCancel={() => setAddingSlot(false)} onSubmit={addSlot} />
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

      {/* A rail, not a modal. Inspecting is something you do *while* comparing slots, so it
          must not cover the grid or need dismissing before the next click. */}
      {selectedNode && <ItemDetailPanel node={selectedNode} onClose={() => setSelectedEntryId(null)} />}

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
  custom,
  selectedEntryId,
  onPick,
  onZoom,
  onRemove,
  onSelect,
  onRemoveSlot,
}: {
  slot: SlotDefinition;
  occupants: ResolvedEntry[];
  readOnly: boolean;
  /** True if the owner invented this slot rather than inheriting it from the template. */
  custom?: boolean;
  selectedEntryId: string | null;
  onPick: () => void;
  onZoom: (node: ResolvedEntry) => void;
  onRemove: (node: ResolvedEntry) => void;
  onSelect: (entryId: string | null) => void;
  /** Absent for template slots, which are not the owner's to delete. */
  onRemoveSlot?: () => void;
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
      <div className={`flex items-center gap-2 px-3 py-2 ${custom ? 'bg-sky-500/10' : 'bg-black/20'}`}>
        <span className="text-[10px] font-bold text-stone-500 uppercase tracking-widest truncate flex-1">
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
        {/* Three distinct states. Previously "full" and "read-only" both rendered as a
            bare absence of the + button, which reads as a broken slot either way. */}
        {!readOnly && !full && (
          <button onClick={onPick} className="text-stone-600 hover:text-orange-400 shrink-0" title="Add item">
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
        {onRemoveSlot && (
          <button
            onClick={onRemoveSlot}
            className="text-stone-600 hover:text-red-400 shrink-0"
            title="Remove this slot"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
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
            const selected = selectedEntryId === node.entry.id;
            return (
              <div
                key={node.entry.id}
                className={`group rounded-lg -mx-1 px-1 ${selected ? 'bg-sky-500/10 ring-1 ring-sky-500/40' : ''}`}
              >
                <div className="flex items-start gap-2">
                  <ItemThumb imageUrl={node.item.image_url} category={node.item.category} size="w-10 h-10" />
                  {/* Clicking the item is inspect, not edit. A second click closes, so the
                      rail is a toggle rather than something you have to go dismiss. */}
                  <button
                    onClick={() => onSelect(selected ? null : node.entry.id)}
                    className="flex-1 min-w-0 text-left"
                    title="Show details"
                  >
                    <div className={`text-sm leading-tight ${selected ? 'text-sky-200' : 'text-stone-200'}`}>
                      {node.item.name}
                      {node.entry.quantity > 1 && (
                        <span className="text-stone-500 font-mono text-xs"> ×{node.entry.quantity}</span>
                      )}
                    </div>
                    <div className="flex gap-2 text-[10px] font-mono text-stone-500 mt-1">
                      <span>{formatGrams(Number(node.item.metadata?.core?.weight_g ?? 0) * node.entry.quantity)}</span>
                      <span>{formatCost(Number(node.item.metadata?.core?.cost_cents ?? 0) * node.entry.quantity)}</span>
                    </div>
                  </button>
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

/**
 * Inline form for inventing a slot, sized to sit in the grid where the "Add slot" tile was.
 *
 * Inline rather than a modal because it is a small, low-stakes, repeatable action, and
 * because you want to see the slots you already have while naming the next one.
 */
function AddSlotForm({
  busy,
  onCancel,
  onSubmit,
}: {
  busy: boolean;
  onCancel: () => void;
  onSubmit: (name: string, category: string) => void;
}) {
  const [name, setName] = useState('');
  const [category, setCategory] = useState('');

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (name.trim()) onSubmit(name.trim(), category);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onCancel();
      }}
      className="rounded-xl border border-sky-500/40 bg-stone-900 p-3 flex flex-col gap-2 min-h-[9rem]"
    >
      <div className="text-[10px] font-bold uppercase tracking-widest text-sky-400/80">New slot</div>
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Slot name"
        className="w-full px-2 py-1.5 rounded-lg bg-stone-950 border border-stone-800 text-sm text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-sky-500/60"
      />
      <select
        value={category}
        onChange={(e) => setCategory(e.target.value)}
        className="w-full px-2 py-1.5 rounded-lg bg-stone-950 border border-stone-800 text-xs text-stone-300 focus:outline-none focus:border-sky-500/60"
      >
        {/* Empty means universal. Restricting is the opt-in, because a slot you invented for
            yourself is usually for the one thing the template did not anticipate. */}
        <option value="">Accepts anything</option>
        {SLOT_CATEGORIES.map((c) => (
          <option key={c} value={c}>
            Only {c}
          </option>
        ))}
      </select>
      <div className="flex gap-2 mt-auto">
        <button
          type="submit"
          disabled={busy || !name.trim()}
          className="flex-1 px-3 py-1.5 rounded-lg bg-sky-500/20 hover:bg-sky-500/30 text-sky-200 text-xs font-medium disabled:opacity-40"
        >
          Add
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-3 py-1.5 rounded-lg bg-stone-800 hover:bg-stone-700 text-xs text-stone-400"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

/**
 * Categories a custom slot may restrict itself to.
 *
 * Hard-coded against the backend's seeded vocabulary rather than derived from the item
 * catalogue: a category with nothing in it yet is still a legitimate thing to reserve a slot
 * for, and deriving the list would quietly hide exactly those.
 */
const SLOT_CATEGORIES = [
  'consumable',
  'electronics',
  'fuel',
  'kitchen',
  'organizer',
  'outerwear',
  'pack',
  'pants',
  'poles',
  'shelter',
  'shirt',
  'shoes',
  'sleep',
];
