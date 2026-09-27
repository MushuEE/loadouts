import { useState } from 'react';
import { Check, ImageOff } from 'lucide-react';
import { isAcceptableImageURL } from '../lib/covers';

/**
 * Choose an imported item's thumbnail from the images the page offered, or paste another.
 *
 * Retailers list a lifestyle shot or a size chart first as often as the product itself, so
 * defaulting to the first image and giving no choice leaves a lot of kit pictured by a
 * model's back. Candidates that fail to load are dropped from the grid rather than shown
 * as broken tiles; a store that blocks hotlinking would otherwise fill it with them.
 */
export function ThumbnailPicker({
  candidates,
  value,
  onChange,
}: {
  candidates: string[];
  value: string;
  onChange: (url: string) => void;
}) {
  const [broken, setBroken] = useState<Set<string>>(new Set());
  const [custom, setCustom] = useState(value && !candidates.includes(value) ? value : '');
  const customValid = isAcceptableImageURL(custom.trim());
  const visible = candidates.filter((c) => !broken.has(c));

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-6 gap-1.5">
        <Tile selected={value === ''} onClick={() => onChange('')} title="No image (use the category icon)">
          <ImageOff className="w-4 h-4 text-stone-600" />
        </Tile>
        {visible.map((url) => (
          <Tile key={url} selected={value === url} onClick={() => onChange(url)} title={url} light>
            <img
              src={url}
              alt=""
              referrerPolicy="no-referrer"
              onError={() => {
                setBroken((b) => new Set(b).add(url));
                if (value === url) onChange('');
              }}
              className="w-full h-full object-contain"
            />
          </Tile>
        ))}
      </div>
      <div className="flex gap-2">
        <input
          value={custom}
          onChange={(e) => setCustom(e.target.value)}
          placeholder="…or paste an image URL"
          className={`flex-1 min-w-0 bg-black/40 border rounded-md px-2 py-1 text-xs text-stone-300 outline-none placeholder:text-stone-600 ${
            customValid ? 'border-stone-800 focus:border-orange-500' : 'border-red-500/60'
          }`}
        />
        <button
          type="button"
          disabled={!custom.trim() || !customValid}
          onClick={() => onChange(custom.trim())}
          className="px-2.5 py-1 rounded-md bg-stone-800 hover:bg-stone-700 disabled:opacity-40 text-xs text-stone-200"
        >
          Use
        </button>
      </div>
    </div>
  );
}

function Tile({
  selected,
  onClick,
  title,
  light,
  children,
}: {
  selected: boolean;
  onClick: () => void;
  title: string;
  light?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={title}
      className={`relative aspect-square rounded-md overflow-hidden border flex items-center justify-center ${
        light ? 'bg-stone-100' : 'bg-black/40'
      } ${selected ? 'border-orange-500 ring-1 ring-orange-500' : 'border-stone-800 hover:border-stone-600'}`}
    >
      {children}
      {selected && (
        <span className="absolute top-0.5 right-0.5 rounded-full bg-orange-500 p-0.5">
          <Check className="w-2.5 h-2.5 text-white" />
        </span>
      )}
    </button>
  );
}
