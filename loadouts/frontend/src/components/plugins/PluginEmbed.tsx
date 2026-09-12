import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { RenderedView } from '../../api/types';

/**
 * Hosts one embed view in a sandboxed iframe.
 *
 * The sandbox attribute is the whole security model and its exact value matters:
 * `allow-scripts` WITHOUT `allow-same-origin`. Adding the second alongside the first
 * silently voids the sandbox, because the frame could then reach into its own origin and
 * remove the attribute. Leaving it off gives the frame an opaque origin, so it has no
 * cookies, no localStorage, and no access to this page.
 *
 * That also means the frame has no credentials and cannot call the API itself. Everything
 * privileged is a postMessage to this component, which performs it as the signed-in user
 * and posts the result back. The plugin can ask; we decide.
 */
export function PluginEmbed({ view, scope }: { view: RenderedView; scope: { type: string; id: string } }) {
  const frameRef = useRef<HTMLIFrameElement | null>(null);
  const [height, setHeight] = useState(view.height || 320);
  const [failure, setFailure] = useState<string | null>(null);

  useEffect(() => {
    function post(message: Record<string, unknown>) {
      frameRef.current?.contentWindow?.postMessage({ source: 'loadouts-host', ...message }, '*');
    }

    async function onMessage(event: MessageEvent) {
      // Only listen to our own frame. The frame's origin is opaque ("null"), so there is
      // no origin to check against; identity comes from the window reference instead.
      if (!frameRef.current || event.source !== frameRef.current.contentWindow) return;

      const msg = event.data;
      if (!msg || msg.source !== 'loadouts-plugin') return;

      if (msg.type === 'ready') {
        post({ type: 'context', payload: view.embed_context ?? {} });
        return;
      }

      try {
        switch (msg.type) {
          case 'resize': {
            const requested = Number(msg.payload?.height);
            // Clamped: a plugin should not be able to push the rest of the page off
            // screen, deliberately or by miscalculating.
            if (Number.isFinite(requested)) setHeight(Math.min(Math.max(requested, 80), 900));
            post({ id: msg.id, payload: { ok: true } });
            break;
          }
          case 'data.put': {
            const datum = await api.putPluginDatum(view.plugin_id, String(msg.payload?.key ?? ''), {
              scope_type: scope.type,
              scope_id: scope.id,
              value: msg.payload?.value ?? {},
            });
            post({ id: msg.id, payload: datum });
            break;
          }
          case 'data.delete': {
            await api.deletePluginDatum(view.plugin_id, String(msg.payload?.key ?? ''), {
              scope_type: scope.type,
              scope_id: scope.id,
            });
            post({ id: msg.id, payload: { ok: true } });
            break;
          }
          default:
            post({ id: msg.id, error: `unsupported request "${msg.type}"` });
        }
      } catch (err) {
        // The failure goes back to the plugin rather than to the console, so an author
        // can surface a sensible message instead of silently doing nothing.
        post({ id: msg.id, error: err instanceof Error ? err.message : 'request failed' });
      }
    }

    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [view, scope.type, scope.id]);

  if (!view.embed_url) {
    return <p className="text-sm text-red-300">This embed has no sandbox URL.</p>;
  }
  if (failure) {
    return <p className="text-sm text-red-300">{failure}</p>;
  }

  return (
    <iframe
      ref={frameRef}
      src={view.embed_url}
      title={view.title}
      height={height}
      // allow-scripts alone. See the comment above: adding allow-same-origin here would
      // void the isolation entirely.
      sandbox="allow-scripts"
      referrerPolicy="no-referrer"
      loading="lazy"
      onError={() => setFailure('This plugin failed to load.')}
      className="w-full rounded-lg border border-stone-800 bg-stone-950"
    />
  );
}
