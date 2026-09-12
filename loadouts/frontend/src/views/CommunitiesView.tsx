import { useState } from 'react';
import { Plus, Users } from 'lucide-react';
import { api } from '../api/client';
import type { Community, CommunityMember, CommunityView, Item, LoadoutSummary, TemplateDetail } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { Badge, EmptyState, ErrorNote, LoadoutCard, Spinner } from '../components/ui';

export function CommunitiesView({ onOpenLoadout }: { onOpenLoadout: (id: string) => void }) {
  const [slug, setSlug] = useState<string | null>(null);
  const communities = useAsync<Community[]>(() => api.listCommunities(), []);

  return (
    <div className="flex-1 flex overflow-hidden bg-stone-950 text-white">
      <aside className="w-72 border-r border-stone-900 overflow-y-auto p-4 shrink-0">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-bold uppercase tracking-wider text-stone-500">Communities</h2>
          <CreateCommunityButton onCreated={() => communities.reload()} />
        </div>
        {communities.loading && <Spinner />}
        {communities.error && <ErrorNote message={communities.error} />}
        <div className="space-y-1">
          {communities.data?.map((c) => (
            <button
              key={c.id}
              onClick={() => setSlug(c.slug)}
              className={`w-full text-left p-3 rounded-lg transition-colors ${
                slug === c.slug ? 'bg-stone-800 text-white' : 'text-stone-400 hover:bg-stone-900'
              }`}
            >
              <div className="text-sm font-medium">{c.name}</div>
              <div className="text-[11px] text-stone-600">
                /{c.slug} · {c.member_count} member{c.member_count === 1 ? '' : 's'}
              </div>
            </button>
          ))}
        </div>
      </aside>

      <div className="flex-1 overflow-y-auto p-10">
        {slug ? (
          <CommunityDetail slug={slug} onOpenLoadout={onOpenLoadout} onMembershipChange={() => communities.reload()} />
        ) : (
          <div className="h-full flex flex-col items-center justify-center text-stone-700">
            <Users className="w-12 h-12 mb-3" />
            <p className="text-sm">Pick a community to see its templates, loadouts, and item layer.</p>
          </div>
        )}
      </div>
    </div>
  );
}

function CreateCommunityButton({ onCreated }: { onCreated: () => void }) {
  const [busy, setBusy] = useState(false);

  async function create() {
    const name = window.prompt('Community name (e.g. "Bikepacking")');
    if (!name) return;
    setBusy(true);
    try {
      await api.createCommunity({ name });
      onCreated();
    } catch (err) {
      window.alert((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      onClick={create}
      disabled={busy}
      title="Create community"
      className="p-1 rounded text-stone-500 hover:text-orange-400 disabled:opacity-50"
    >
      <Plus className="w-4 h-4" />
    </button>
  );
}

function CommunityDetail({
  slug,
  onOpenLoadout,
  onMembershipChange,
}: {
  slug: string;
  onOpenLoadout: (id: string) => void;
  onMembershipChange: () => void;
}) {
  const view = useAsync<CommunityView>(() => api.getCommunity(slug), [slug]);
  const members = useAsync<CommunityMember[]>(() => api.communityMembers(slug), [slug]);
  const templates = useAsync<TemplateDetail[]>(() => api.communityTemplates(slug), [slug]);
  const loadouts = useAsync<LoadoutSummary[]>(() => api.communityLoadouts(slug), [slug]);

  if (view.loading) return <Spinner />;
  if (view.error) return <ErrorNote message={view.error} />;
  if (!view.data) return null;

  const { community, viewer_role } = view.data;
  const isMember = viewer_role !== '';
  const isAdmin = viewer_role === 'admin' || viewer_role === 'owner';

  async function toggleMembership() {
    try {
      if (isMember) {
        await api.leaveCommunity(slug);
      } else {
        await api.joinCommunity(slug);
      }
      view.reload();
      members.reload();
      onMembershipChange();
    } catch (err) {
      window.alert((err as Error).message);
    }
  }

  return (
    <div className="max-w-4xl">
      <div className="flex items-start justify-between gap-6">
        <div>
          <h1 className="text-3xl font-light">{community.name}</h1>
          <p className="text-stone-500 text-sm mt-1">{community.description}</p>
          <div className="flex items-center gap-2 mt-3">
            <Badge className="bg-stone-800 text-stone-400">/{community.slug}</Badge>
            <Badge className="bg-stone-800 text-stone-400">{view.data.member_count} members</Badge>
            {viewer_role && <Badge className="bg-orange-500/15 text-orange-300">you: {viewer_role}</Badge>}
          </div>
        </div>
        <button
          onClick={toggleMembership}
          className={`px-4 py-2 rounded-lg text-sm font-medium ${
            isMember ? 'bg-stone-800 hover:bg-stone-700 text-stone-300' : 'bg-orange-500 hover:bg-orange-400 text-white'
          }`}
        >
          {isMember ? 'Leave' : 'Join'}
        </button>
      </div>

      {isAdmin && <CommunityLayerEditor slug={slug} />}

      <Section title="Templates" hint="Scaffolding this community maintains for its members.">
        {templates.data?.length === 0 && <EmptyState title="No community templates yet" />}
        <div className="grid grid-cols-2 gap-3">
          {templates.data?.map((t) => (
            <div key={t.template.id} className="p-4 rounded-xl bg-stone-900 border border-stone-800">
              <div className="font-medium text-stone-200">{t.template.name}</div>
              <div className="text-[11px] text-stone-500 mt-1">
                v{t.template.latest_version} · {t.version.slots.length} slots
              </div>
              <div className="text-[11px] text-stone-600 mt-2">
                {t.version.slots.map((s) => s.name).join(', ') || 'No fixed structure'}
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Loadouts">
        {loadouts.loading && <Spinner />}
        {loadouts.data?.length === 0 && <EmptyState title="No loadouts posted here yet" />}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {loadouts.data?.map((summary) => (
            <LoadoutCard
              key={summary.loadout.id}
              summary={summary}
              onOpen={() => onOpenLoadout(summary.loadout.id)}
              onFork={async () => {
                try {
                  const detail = await api.forkLoadout(summary.loadout.id);
                  onOpenLoadout(detail.loadout.id);
                } catch (err) {
                  window.alert((err as Error).message);
                }
              }}
            />
          ))}
        </div>
      </Section>

      <Section title="Members">
        <div className="flex flex-wrap gap-2">
          {members.data?.map((m) => (
            <span key={m.profile.id} className="px-3 py-1.5 rounded-lg bg-stone-900 border border-stone-800 text-xs">
              <span className="text-stone-300">@{m.profile.handle}</span>
              <span className="text-stone-600"> · {m.membership.role}</span>
            </span>
          ))}
        </div>
      </Section>
    </div>
  );
}

function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="mt-10">
      <h2 className="text-sm font-bold uppercase tracking-wider text-stone-500">{title}</h2>
      {hint && <p className="text-[11px] text-stone-600 mt-1 mb-3">{hint}</p>}
      <div className="mt-3">{children}</div>
    </div>
  );
}

/**
 * The community metadata layer editor. This is how a community extends a *global* item with
 * its own scoped attributes (the UL Backpacking {ul_score, comfort, durability} example)
 * without mutating the shared product record.
 */
function CommunityLayerEditor({ slug }: { slug: string }) {
  const items = useAsync<Item[]>(() => api.listItems(), []);
  const [itemId, setItemId] = useState('');
  const [namespace, setNamespace] = useState(slug.replace(/-/g, '_'));
  const [json, setJson] = useState('{\n  "ul_score": 8.0,\n  "comfort": 7.0\n}');
  const [status, setStatus] = useState<string | null>(null);

  async function save() {
    setStatus(null);
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(json);
    } catch {
      setStatus('That is not valid JSON.');
      return;
    }
    if (!itemId) {
      setStatus('Pick an item first.');
      return;
    }
    try {
      await api.setCommunityItemLayer(slug, itemId, { [namespace]: parsed });
      setStatus(`Saved. Members viewing that item in /${slug} now see these values.`);
    } catch (err) {
      setStatus((err as Error).message);
    }
  }

  return (
    <div className="mt-8 p-5 rounded-xl bg-sky-500/5 border border-sky-500/20">
      <h2 className="text-sm font-bold uppercase tracking-wider text-sky-300">Community item layer</h2>
      <p className="text-[11px] text-stone-500 mt-1">
        Admin only. Adds community-scoped metadata on top of the global item — it never changes the
        global product, and it only appears when the item is viewed in this community.
      </p>

      <div className="grid grid-cols-2 gap-3 mt-4">
        <select
          value={itemId}
          onChange={(e) => setItemId(e.target.value)}
          className="bg-black/40 border border-stone-700 rounded-lg p-2.5 text-sm text-white outline-none"
        >
          <option value="">Select an item…</option>
          {items.data?.map((i) => (
            <option key={i.id} value={i.id}>
              {i.name}
            </option>
          ))}
        </select>
        <input
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
          placeholder="namespace"
          className="bg-black/40 border border-stone-700 rounded-lg p-2.5 text-sm text-white outline-none"
        />
      </div>

      <textarea
        value={json}
        onChange={(e) => setJson(e.target.value)}
        rows={5}
        spellCheck={false}
        className="w-full mt-3 bg-black/40 border border-stone-700 rounded-lg p-3 text-sm font-mono text-sky-200 outline-none"
      />

      <div className="flex items-center gap-3 mt-3">
        <button
          onClick={save}
          className="px-4 py-2 rounded-lg bg-sky-500/20 hover:bg-sky-500/30 text-sky-200 text-sm font-medium"
        >
          Save layer
        </button>
        {status && <span className="text-xs text-stone-400">{status}</span>}
      </div>
    </div>
  );
}
