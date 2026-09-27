import { useEffect, useRef, useState } from 'react';
import { Check, ChevronsUpDown } from 'lucide-react';
import type { Profile } from '../api/types';
import { useSession } from '../session/SessionContext';
import { Badge } from './ui';

/**
 * Who you are acting as. Everything the app shows (ownership, private notes, community
 * roles, edit rights) depends on the active profile, so it gets one clear control rather
 * than a stack of look-alike nav buttons.
 *
 * There is no real sign-in yet: the API lists every profile and trusts X-Profile-ID, so
 * this menu doubles as a dev tool for acting as anyone. Profiles are grouped by the user
 * who owns them, which is what separates "my other profile" from "someone else".
 */
export function ProfileSwitcher() {
  const session = useSession();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const active = session.profile;
  const mine = session.profiles.filter((p) => active && p.user_id === active.user_id);
  const others = session.profiles.filter((p) => !active || p.user_id !== active.user_id);

  function pick(id: string) {
    session.switchProfile(id);
    setOpen(false);
  }

  return (
    <div ref={root} className="relative w-full px-2 pb-1 flex flex-col items-center">
      <button
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        title={active ? `Acting as ${active.display_name} (@${active.handle}). Click to switch.` : 'Choose a profile'}
        className={`w-full flex flex-col items-center gap-1 rounded-xl py-1.5 transition-colors ${
          open ? 'bg-stone-800' : 'hover:bg-stone-900'
        }`}
      >
        {active ? <Avatar profile={active} size="lg" /> : <div className="w-10 h-10 rounded-full bg-stone-800" />}
        <span className="flex items-center gap-0.5 max-w-full text-[10px] text-stone-400">
          <span className="truncate">{active ? `@${active.handle}` : 'No profile'}</span>
          <ChevronsUpDown size={10} className="shrink-0" />
        </span>
      </button>

      {open && (
        <div
          role="menu"
          className="absolute left-full bottom-0 ml-2 w-72 rounded-xl border border-stone-800 bg-stone-900 shadow-2xl z-50 overflow-hidden"
        >
          <div className="px-4 pt-3 pb-2 border-b border-stone-800">
            <div className="text-sm font-medium text-white">Switch profile</div>
            <div className="text-[11px] text-stone-500">
              What you can see and edit follows the active profile.
            </div>
          </div>

          {mine.length > 0 && (
            <Group label="Your account">
              {mine.map((p) => (
                <Row key={p.id} profile={p} active={p.id === active?.id} onPick={pick} />
              ))}
            </Group>
          )}

          {others.length > 0 && (
            <Group label="Other accounts" note="dev only">
              {others.map((p) => (
                <Row key={p.id} profile={p} active={false} onPick={pick} />
              ))}
            </Group>
          )}

          <div className="px-4 py-2 border-t border-stone-800 text-[10px] text-stone-600">
            No sign-in yet: this switcher stands in for it during development.
          </div>
        </div>
      )}
    </div>
  );
}

function Group({ label, note, children }: { label: string; note?: string; children: React.ReactNode }) {
  return (
    <div className="py-1.5">
      <div className="px-4 py-1 flex items-center gap-2 text-[10px] font-bold uppercase tracking-wider text-stone-500">
        {label}
        {note && <span className="font-normal normal-case tracking-normal text-stone-600">({note})</span>}
      </div>
      {children}
    </div>
  );
}

function Row({ profile, active, onPick }: { profile: Profile; active: boolean; onPick: (id: string) => void }) {
  return (
    <button
      role="menuitemradio"
      aria-checked={active}
      onClick={() => onPick(profile.id)}
      className={`w-full flex items-center gap-3 px-4 py-2 text-left transition-colors ${
        active ? 'bg-orange-500/10' : 'hover:bg-stone-800'
      }`}
    >
      <Avatar profile={profile} size="sm" />
      <div className="min-w-0 flex-1">
        <div className="text-sm text-white truncate">{profile.display_name}</div>
        <div className="flex items-center gap-1.5 text-[11px] text-stone-500">
          <span className="truncate">@{profile.handle}</span>
          {profile.is_site_admin && (
            <Badge className="bg-sky-500/15 text-sky-300" title="Can place figures in paperdoll layouts">
              Admin
            </Badge>
          )}
          {profile.is_sponsor && <Badge className="bg-amber-500/15 text-amber-300">Sponsor</Badge>}
        </div>
      </div>
      {active && <Check size={16} className="text-orange-400 shrink-0" />}
    </button>
  );
}

/** A profile picture, or initials from the display name when there isn't one. */
function Avatar({ profile, size }: { profile: Profile; size: 'sm' | 'lg' }) {
  const box = size === 'lg' ? 'w-10 h-10 text-sm' : 'w-8 h-8 text-xs';
  if (profile.avatar_url) {
    return <img src={profile.avatar_url} alt="" referrerPolicy="no-referrer" className={`${box} rounded-full object-cover`} />;
  }
  return (
    <div className={`${box} rounded-full bg-orange-500 text-white font-bold flex items-center justify-center shrink-0`}>
      {initials(profile.display_name || profile.handle)}
    </div>
  );
}

/** "Alex | Fit Check" -> "AF", "Trail Sponsor" -> "TS". */
function initials(name: string): string {
  const words = name.split(/[^\p{L}\p{N}]+/u).filter(Boolean);
  return (words.length > 1 ? words[0][0] + words[1][0] : name.slice(0, 2)).toUpperCase();
}
