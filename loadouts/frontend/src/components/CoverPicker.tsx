import { useEffect, useState } from 'react';
import { Check, ImageOff } from 'lucide-react';
import { COVER_PRESETS, isAcceptableImageURL } from '../lib/covers';

/**
 * Choose a loadout cover: one of the presets, a custom image URL, or none.
 *
 * Controlled: `value` is the cover_image_url itself, so the picker works identically
 * for a loadout being created and one being edited.
 */
export function CoverPicker({
  value,
  onChange,
  columns = 'grid-cols-3 sm:grid-cols-5',
}: {
  value: string;
  onChange: (url: string) => void;
  /** Grid column classes; a narrow sidebar needs fewer than a modal. */
  columns?: string;
}) {
  const isPreset = COVER_PRESETS.some((p) => p.url === value);
  const [custom, setCustom] = useState(!isPreset && value ? value : '');
  const customValid = isAcceptableImageURL(custom.trim());

  return (
    <div className="space-y-2">
      <div className={`grid ${columns} gap-2`}>
        <button
          type="button"
          onClick={() => onChange('')}
          title="No cover"
          className={`relative aspect-video rounded-md border flex items-center justify-center text-stone-600 ${
            value === '' ? 'border-orange-500 ring-1 ring-orange-500' : 'border-stone-800 hover:border-stone-600'
          }`}
        >
          <ImageOff className="w-4 h-4" />
        </button>
        {COVER_PRESETS.map((p) => (
          <button
            type="button"
            key={p.url}
            onClick={() => onChange(p.url)}
            title={p.label}
            className={`relative aspect-video rounded-md overflow-hidden border ${
              value === p.url ? 'border-orange-500 ring-1 ring-orange-500' : 'border-stone-800 hover:border-stone-600'
            }`}
          >
            <img src={p.url} alt={p.label} className="w-full h-full object-cover" />
            {value === p.url && (
              <Check className="absolute top-1 right-1 w-3.5 h-3.5 text-white drop-shadow" />
            )}
          </button>
        ))}
      </div>
      <div className="flex gap-2">
        <input
          value={custom}
          onChange={(e) => setCustom(e.target.value)}
          placeholder="…or paste an image URL"
          className={`flex-1 min-w-0 bg-black/40 border rounded-lg px-3 py-1.5 text-xs text-white outline-none placeholder:text-stone-600 ${
            customValid ? 'border-stone-800 focus:border-orange-500' : 'border-red-500/60'
          }`}
        />
        <button
          type="button"
          disabled={!custom.trim() || !customValid}
          onClick={() => onChange(custom.trim())}
          className="px-3 py-1.5 rounded-lg bg-stone-800 hover:bg-stone-700 disabled:opacity-40 text-xs text-stone-200"
        >
          Use
        </button>
      </div>
      {!customValid && <p className="text-[11px] text-red-400">Use an http(s) image link.</p>}
    </div>
  );
}

/** A cover banner that quietly disappears when the image can't load. */
export function CoverImage({ url, className = '' }: { url: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [url]);
  if (!url || failed) return null;
  return (
    <img
      src={url}
      alt=""
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
      className={`w-full object-cover bg-stone-800 ${className}`}
    />
  );
}
