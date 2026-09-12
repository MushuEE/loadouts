import { useState } from 'react';
import { AlertTriangle, Check, Power, Puzzle, Search, Shield, Trash2, X } from 'lucide-react';
import { api } from '../api/client';
import type { PluginDetail, PluginInstallView, PluginManifest } from '../api/types';
import { useSession } from '../session/SessionContext';
import { useAsync } from '../lib/useAsync';
import { ErrorNote, Spinner } from '../components/ui';

/**
 * The plugin manager: browse what exists, see exactly what each one wants, install it.
 *
 * Installing is deliberately a consent screen rather than a button. A plugin declares its
 * capabilities in its manifest and the server refuses an install that does not grant all
 * of them, so the user is never in a position where something was enabled with access
 * they did not see.
 */
export function PluginsView() {
  const session = useSession();
  const [query, setQuery] = useState('');
  const [installing, setInstalling] = useState<PluginDetail | null>(null);

  const directory = useAsync(() => api.listPlugins({ q: query || undefined }), [query]);
  const installs = useAsync(
    () => (session.profile ? api.listInstalls({ scope_type: 'profile' }) : Promise.resolve({ installs: [] })),
    [session.profile?.id],
  );

  const installedIds = new Set((installs.data?.installs ?? []).map((i) => i.plugin.id));

  return (
    <div className="flex-1 overflow-y-auto bg-stone-950">
      <div className="max-w-5xl mx-auto p-8">
        <header className="mb-6">
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Puzzle className="text-orange-400" /> Plugins
          </h1>
          <p className="text-sm text-stone-500 mt-1">
            Charts, calculators, tables, and maps that communities build for their own way of
            looking at gear.
          </p>
        </header>

        <section className="mb-8">
          <h2 className="text-xs uppercase tracking-wide text-stone-500 mb-2">Installed</h2>
          {installs.loading && <Spinner />}
          {installs.error && <ErrorNote message={installs.error} />}
          {!installs.loading && (installs.data?.installs.length ?? 0) === 0 && (
            <p className="text-sm text-stone-600 italic">
              Nothing installed yet. Anything you add below shows up on your loadouts.
            </p>
          )}
          <div className="space-y-2">
            {(installs.data?.installs ?? []).map((view) => (
              <InstalledRow key={view.install.id} view={view} onChanged={installs.reload} />
            ))}
          </div>
        </section>

        <section>
          <div className="flex items-center gap-2 mb-3">
            <h2 className="text-xs uppercase tracking-wide text-stone-500 flex-1">Directory</h2>
            <div className="relative">
              <Search className="w-3.5 h-3.5 absolute left-2.5 top-2 text-stone-600" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search plugins"
                className="bg-stone-900 border border-stone-800 rounded-lg pl-8 pr-3 py-1.5 text-sm text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-orange-500/60"
              />
            </div>
          </div>

          {directory.loading && <Spinner />}
          {directory.error && <ErrorNote message={directory.error} />}
          <div className="grid md:grid-cols-2 gap-3">
            {(directory.data?.plugins ?? []).map((detail) => (
              <DirectoryCard
                key={detail.plugin.id}
                detail={detail}
                installed={installedIds.has(detail.plugin.id)}
                onInstall={() => setInstalling(detail)}
              />
            ))}
          </div>
          {!directory.loading && (directory.data?.plugins.length ?? 0) === 0 && (
            <p className="text-sm text-stone-600 italic">No plugins match that search.</p>
          )}
        </section>
      </div>

      {installing && (
        <ConsentModal
          detail={installing}
          onClose={() => setInstalling(null)}
          onInstalled={() => {
            setInstalling(null);
            installs.reload();
          }}
        />
      )}
    </div>
  );
}

function DirectoryCard({
  detail,
  installed,
  onInstall,
}: {
  detail: PluginDetail;
  installed: boolean;
  onInstall: () => void;
}) {
  const views = detail.version.manifest.views ?? [];
  return (
    <article className="bg-stone-900 border border-stone-800 rounded-xl p-4 flex flex-col">
      <div className="flex items-start gap-2">
        <h3 className="font-semibold text-stone-100 flex-1">{detail.plugin.name}</h3>
        <span className="text-[10px] text-stone-600">v{detail.plugin.latest_version}</span>
      </div>
      <p className="text-xs text-stone-500 mt-1 flex-1">{detail.plugin.description || 'No description.'}</p>

      <div className="flex flex-wrap gap-1 mt-3">
        {views.map((view) => (
          <span
            key={view.id}
            className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-400"
            title={`${view.kind} on ${view.surface}`}
          >
            {view.surface}
          </span>
        ))}
      </div>

      <button
        onClick={onInstall}
        disabled={installed}
        className="mt-3 w-full px-3 py-1.5 rounded-lg text-sm font-medium bg-orange-500 hover:bg-orange-400 text-white disabled:bg-stone-800 disabled:text-stone-500"
      >
        {installed ? 'Installed' : 'Install'}
      </button>
    </article>
  );
}

function InstalledRow({ view, onChanged }: { view: PluginInstallView; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="bg-stone-900 border border-stone-800 rounded-xl p-3">
      <div className="flex items-center gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-stone-100 truncate">{view.plugin.name}</span>
            <span className="text-[10px] text-stone-600">v{view.install.version}</span>
            {!view.install.enabled && (
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-500">paused</span>
            )}
          </div>
          <div className="flex flex-wrap gap-1 mt-1">
            {view.install.granted_caps.map((cap) => (
              <span key={cap} className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-500">
                {cap}
              </span>
            ))}
          </div>
        </div>

        <button
          onClick={() => run(() => api.updateInstall(view.install.id, { enabled: !view.install.enabled }))}
          disabled={busy}
          title={view.install.enabled ? 'Pause' : 'Resume'}
          className="p-2 rounded-lg bg-stone-800 hover:bg-stone-700 text-stone-400 disabled:opacity-50"
        >
          <Power size={14} />
        </button>
        <button
          onClick={() => run(() => api.uninstallPlugin(view.install.id))}
          disabled={busy}
          title="Uninstall"
          className="p-2 rounded-lg bg-stone-800 hover:bg-red-900/50 text-stone-400 hover:text-red-300 disabled:opacity-50"
        >
          <Trash2 size={14} />
        </button>
      </div>

      {/* An upgrade that needs more access is surfaced rather than applied. The version
          stays pinned until the user agrees to the new ask. */}
      {view.upgrade_available && (
        <div className="mt-2 text-[11px] flex items-start gap-1.5 text-amber-300/80">
          <AlertTriangle size={12} className="mt-0.5 shrink-0" />
          <span>
            Version {view.plugin.latest_version} is available.
            {view.missing_caps?.length
              ? ` It also wants ${view.missing_caps.join(', ')}, so upgrading needs your approval.`
              : ' '}
          </span>
        </div>
      )}
      {error && <p className="mt-2 text-[11px] text-red-300">{error}</p>}
    </div>
  );
}

/**
 * The install consent screen. It lists every capability and collects any settings the
 * manifest declares, because the server will refuse a partial grant.
 */
function ConsentModal({
  detail,
  onClose,
  onInstalled,
}: {
  detail: PluginDetail;
  onClose: () => void;
  onInstalled: () => void;
}) {
  const session = useSession();
  const manifest: PluginManifest = detail.version.manifest;
  const capabilities = describeCapabilities(manifest);
  const settingDefs = manifest.settings ?? [];

  const [settings, setSettings] = useState<Record<string, string>>(() =>
    Object.fromEntries(settingDefs.map((s) => [s.key, s.default ?? ''])),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function install() {
    if (!session.profile) {
      setError('Pick a profile first.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await api.installPlugin({
        plugin_id: detail.plugin.id,
        scope_type: 'profile',
        scope_id: session.profile.id,
        granted_caps: capabilities.map((c) => c.id),
        settings,
      });
      onInstalled();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Install failed.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-6">
      <div className="bg-stone-900 border border-stone-800 rounded-2xl w-full max-w-md max-h-[85vh] overflow-y-auto">
        <header className="flex items-center gap-2 p-4 border-b border-stone-800">
          <Shield size={16} className="text-orange-400" />
          <h2 className="font-semibold text-stone-100 flex-1">Install {detail.plugin.name}</h2>
          <button onClick={onClose} className="p-1 text-stone-500 hover:text-white">
            <X size={16} />
          </button>
        </header>

        <div className="p-4 space-y-4">
          <p className="text-sm text-stone-400">{detail.plugin.description}</p>

          <div>
            <h3 className="text-xs uppercase tracking-wide text-stone-500 mb-2">This plugin will be able to</h3>
            {capabilities.length === 0 ? (
              <p className="text-sm text-stone-500 italic">
                Nothing. It renders without reading any of your data.
              </p>
            ) : (
              <ul className="space-y-1.5">
                {capabilities.map((cap) => (
                  <li key={cap.id} className="flex items-start gap-2 text-sm text-stone-300">
                    <Check size={14} className="text-orange-400 mt-0.5 shrink-0" />
                    <span>{cap.label}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {settingDefs.length > 0 && (
            <div>
              <h3 className="text-xs uppercase tracking-wide text-stone-500 mb-2">Settings</h3>
              <div className="space-y-2">
                {settingDefs.map((def) => (
                  <label key={def.key} className="block">
                    <span className="text-xs text-stone-400">
                      {def.label}
                      {def.required && <span className="text-orange-400"> *</span>}
                    </span>
                    <input
                      type={def.type === 'secret' ? 'password' : def.type === 'number' ? 'number' : 'text'}
                      value={settings[def.key] ?? ''}
                      onChange={(e) => setSettings({ ...settings, [def.key]: e.target.value })}
                      className="mt-1 w-full bg-stone-950 border border-stone-800 rounded-lg px-3 py-1.5 text-sm text-stone-200 focus:outline-none focus:border-orange-500/60"
                    />
                    {def.help && <span className="text-[11px] text-stone-600">{def.help}</span>}
                  </label>
                ))}
              </div>
            </div>
          )}

          {error && <ErrorNote message={error} />}
        </div>

        <footer className="flex gap-2 p-4 border-t border-stone-800">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 rounded-lg bg-stone-800 hover:bg-stone-700 text-sm text-stone-300"
          >
            Cancel
          </button>
          <button
            onClick={install}
            disabled={busy}
            className="flex-1 px-4 py-2 rounded-lg bg-orange-500 hover:bg-orange-400 text-sm font-medium text-white disabled:opacity-50"
          >
            {busy ? 'Installing…' : 'Approve and install'}
          </button>
        </footer>
      </div>
    </div>
  );
}

/**
 * Turns manifest capabilities into the strings the server expects, paired with plain
 * language. The ids must match core.Capabilities.List() exactly - the server refuses an
 * install whose grants do not cover the ask.
 */
function describeCapabilities(manifest: PluginManifest): { id: string; label: string }[] {
  const caps = manifest.capabilities;
  const out: { id: string; label: string }[] = [];
  if (caps.read_loadout) out.push({ id: 'loadout:read', label: 'Read the loadout it is shown on' });
  if (caps.read_items) out.push({ id: 'items:read', label: 'Read the items in that loadout' });
  if (caps.read_community) out.push({ id: 'community:read', label: 'Read the community catalog' });
  if (caps.storage) out.push({ id: 'storage:write', label: 'Save its own data against your loadouts' });
  for (const host of caps.network ?? []) {
    out.push({ id: `network:${host}`, label: `Connect to ${host}` });
  }
  return out;
}
