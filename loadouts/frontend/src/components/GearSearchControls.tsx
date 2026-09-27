import { useState } from 'react';
import { Hash, Search, X } from 'lucide-react';
import type { GearResult, GearScope, TagCount } from '../api/types';

/** What a search box holds once submitted: free text plus the tags typed as #words. */
export interface SearchTerms {
  q: string;
  tags: string[];
}

export const NO_TERMS: SearchTerms = { q: '', tags: [] };

/** Same rules as the server: lowercased, spaces to hyphens, no leading '#'. */
export function normalizeTag(raw: string): string {
  return raw.trim().replace(/^#+/, '').toLowerCase().split(/\s+/).filter(Boolean).join('-');
}

function addTag(tags: string[], raw: string): string[] {
  const t = normalizeTag(raw);
  return t && !tags.includes(t) ? [...tags, t] : tags;
}

/**
 * A search box where "#springseattle26 hat" means the tag plus the text. Tags lift out into
 * chips as you submit, so what the search is narrowed by is always visible and removable.
 */
export function SearchBar({
  terms,
  onChange,
  placeholder,
}: {
  terms: SearchTerms;
  onChange: (next: SearchTerms) => void;
  placeholder: string;
}) {
  const [draft, setDraft] = useState(terms.q);

  function submit() {
    let tags = terms.tags;
    const words: string[] = [];
    for (const w of draft.split(/\s+/).filter(Boolean)) {
      if (w.startsWith('#')) tags = addTag(tags, w);
      else words.push(w);
    }
    const q = words.join(' ');
    setDraft(q);
    onChange({ q, tags });
  }

  return (
    <div className="flex items-center flex-wrap gap-1.5 bg-stone-900 border border-stone-800 rounded-lg px-3 py-1 focus-within:border-orange-500 flex-1 min-w-[260px]">
      <Search className="w-4 h-4 text-stone-500 shrink-0" />
      {terms.tags.map((t) => (
        <span key={t} className="flex items-center gap-1 pl-2 pr-1 py-0.5 rounded-full bg-orange-500/15 border border-orange-500/40 text-orange-200 text-[11px]">
          #{t}
          <button
            onClick={() => onChange({ ...terms, tags: terms.tags.filter((x) => x !== t) })}
            aria-label={`Remove #${t}`}
            className="text-orange-300/70 hover:text-white"
          >
            <X className="w-3 h-3" />
          </button>
        </span>
      ))}
      <input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') submit();
          if (e.key === 'Backspace' && draft === '' && terms.tags.length > 0) {
            onChange({ ...terms, tags: terms.tags.slice(0, -1) });
          }
        }}
        onBlur={() => draft !== terms.q && submit()}
        placeholder={terms.tags.length ? '' : placeholder}
        className="bg-transparent px-1 py-1 text-sm text-white outline-none flex-1 min-w-[8rem] placeholder:text-stone-600"
      />
    </div>
  );
}

/**
 * Tags you could narrow by next. Several selected tags must all match: a search pins
 * things down, where a loadout's filter bar switches between variants.
 */
export function TagSuggestions({
  suggestions,
  terms,
  onChange,
  unit,
}: {
  suggestions: TagCount[];
  terms: SearchTerms;
  onChange: (next: SearchTerms) => void;
  unit: string;
}) {
  const offered = suggestions.filter((s) => !terms.tags.includes(s.tag));
  if (offered.length === 0) return null;
  return (
    <div className="flex items-center flex-wrap gap-1.5">
      <Hash className="w-3.5 h-3.5 text-stone-600" />
      {offered.map(({ tag, count }) => (
        <button
          key={tag}
          onClick={() => onChange({ ...terms, tags: [...terms.tags, tag] })}
          title={`${count} ${unit}`}
          className="px-2 py-0.5 rounded-full text-[11px] border border-stone-800 text-stone-400 hover:border-stone-600 hover:text-stone-200"
        >
          #{tag}
          <span className="ml-1 tabular-nums text-stone-600">{count}</span>
        </button>
      ))}
    </div>
  );
}

/**
 * An item's tags in a result. Yours are solid; everyone's show how many people use them.
 * Under "mine" only yours are shown, since those are what the search matched.
 */
export function ResultTags({
  result,
  scope,
  onTag,
  max = 4,
}: {
  result: GearResult;
  scope: GearScope;
  onTag?: (tag: string) => void;
  max?: number;
}) {
  const mine = new Set(result.my_tags);
  const rows: { tag: string; count?: number; yours: boolean }[] =
    scope === 'mine'
      ? result.my_tags.map((tag) => ({ tag, yours: true }))
      : result.tags.map(({ tag, count }) => ({ tag, count, yours: mine.has(tag) }));
  if (rows.length === 0) return null;
  const shown = rows.slice(0, max);
  return (
    <span className="flex flex-wrap gap-1">
      {shown.map(({ tag, count, yours }) => (
        <span
          key={tag}
          role={onTag ? 'button' : undefined}
          tabIndex={onTag ? 0 : undefined}
          onClick={(e) => {
            if (!onTag) return;
            e.stopPropagation();
            onTag(tag);
          }}
          onKeyDown={(e) => onTag && e.key === 'Enter' && onTag(tag)}
          title={
            count === undefined
              ? 'Your tag'
              : `${count} ${count === 1 ? 'person' : 'people'} tagged this${yours ? ', you included' : ''}`
          }
          className={`px-1.5 py-px rounded text-[10px] ${
            yours ? 'bg-orange-500/15 text-orange-200' : 'bg-stone-800 text-stone-400'
          } ${onTag ? 'cursor-pointer hover:text-white' : ''}`}
        >
          #{tag}
          {count !== undefined && <span className="ml-1 text-stone-500 tabular-nums">· {count}</span>}
        </span>
      ))}
      {rows.length > max && <span className="text-[10px] text-stone-600">+{rows.length - max}</span>}
    </span>
  );
}
