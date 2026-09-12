import { useState } from 'react';
import { X } from 'lucide-react';
import { api } from '../api/client';
import type { Community, TemplateDetail } from '../api/types';
import { useAsync } from '../lib/useAsync';
import { ErrorNote, Spinner } from './ui';

/**
 * Creating a loadout is explicitly "pick a template, then fill it": the template is the
 * scaffolding, and the Freeform platform template is always available for no structure.
 */
export function NewLoadoutModal({
  communities,
  onClose,
  onCreated,
}: {
  communities: Community[];
  onClose: () => void;
  onCreated: (loadoutId: string) => void;
}) {
  const templates = useAsync<TemplateDetail[]>(() => api.listTemplates(), []);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [templateId, setTemplateId] = useState('platform-freeform');
  const [communityId, setCommunityId] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selected = templates.data?.find((t) => t.template.id === templateId);

  async function submit() {
    if (!name.trim()) {
      setError('Give your loadout a name.');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const detail = await api.createLoadout({
        name,
        description,
        template_id: templateId,
        community_id: communityId,
      });
      onCreated(detail.loadout.id);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-stone-900 border border-stone-700 rounded-xl w-full max-w-2xl max-h-[85vh] flex flex-col">
        <div className="p-4 border-b border-stone-800 flex justify-between items-center">
          <h3 className="font-bold text-white">New Loadout</h3>
          <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-6 space-y-5 overflow-y-auto">
          {error && <ErrorNote message={error} />}

          <div>
            <label className="text-[10px] uppercase font-bold tracking-wider text-stone-500">Name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="PCT Sierra Setup"
              className="w-full mt-1 bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white focus:border-orange-500 outline-none"
            />
          </div>

          <div>
            <label className="text-[10px] uppercase font-bold tracking-wider text-stone-500">Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
              className="w-full mt-1 bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white focus:border-orange-500 outline-none"
            />
          </div>

          <div>
            <label className="text-[10px] uppercase font-bold tracking-wider text-stone-500">Template</label>
            {templates.loading && <Spinner label="Loading templates…" />}
            {templates.error && <ErrorNote message={templates.error} />}
            <div className="mt-2 grid grid-cols-2 gap-2">
              {templates.data?.map((t) => (
                <button
                  key={t.template.id}
                  onClick={() => setTemplateId(t.template.id)}
                  className={`text-left p-3 rounded-lg border transition-colors ${
                    templateId === t.template.id
                      ? 'border-orange-500 bg-orange-500/10'
                      : 'border-stone-800 bg-stone-900/60 hover:border-stone-600'
                  }`}
                >
                  <div className="text-sm font-medium text-stone-200">{t.template.name}</div>
                  <div className="text-[11px] text-stone-500 mt-0.5">
                    v{t.template.latest_version} · {t.version.slots.length} slots · {t.template.owner_type}
                  </div>
                </button>
              ))}
            </div>
            {selected && selected.version.slots.length > 0 && (
              <div className="mt-2 text-[11px] text-stone-500">
                Slots: {selected.version.slots.map((s) => s.name).join(', ')}
              </div>
            )}
          </div>

          <div>
            <label className="text-[10px] uppercase font-bold tracking-wider text-stone-500">
              Post to community (optional)
            </label>
            <select
              value={communityId}
              onChange={(e) => setCommunityId(e.target.value)}
              className="w-full mt-1 bg-black/40 border border-stone-700 rounded-lg p-2.5 text-white focus:border-orange-500 outline-none"
            >
              <option value="">None</option>
              {communities.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
            <p className="text-[11px] text-stone-600 mt-1">
              A community also decides which extra item metadata (like UL scores) is applied.
            </p>
          </div>
        </div>

        <div className="p-4 border-t border-stone-800 flex justify-end gap-3">
          <button onClick={onClose} className="px-4 py-2 text-sm text-stone-400 hover:text-white">
            Cancel
          </button>
          <button
            onClick={submit}
            disabled={saving}
            className="px-5 py-2 rounded-lg bg-orange-500 hover:bg-orange-400 text-white text-sm font-medium disabled:opacity-50"
          >
            {saving ? 'Creating…' : 'Create Loadout'}
          </button>
        </div>
      </div>
    </div>
  );
}
