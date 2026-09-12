import { useState } from 'react';
import { Compass, Database, Home, Loader2, Users } from 'lucide-react';
import { useSession } from './session/SessionContext';
import { HomeView } from './views/HomeView';
import { DiscoverView } from './views/DiscoverView';
import { CommunitiesView } from './views/CommunitiesView';
import { GarageView } from './views/GarageView';
import { LoadoutEditorView } from './views/LoadoutEditorView';

type Tab = 'home' | 'discover' | 'communities' | 'garage';

export default function App() {
  const session = useSession();
  const [tab, setTab] = useState<Tab>('home');
  const [loadoutId, setLoadoutId] = useState<string | null>(null);

  function openLoadout(id: string) {
    setLoadoutId(id);
  }

  if (session.loading) {
    return (
      <div className="h-screen flex items-center justify-center bg-stone-950 text-stone-500 gap-2">
        <Loader2 className="w-4 h-4 animate-spin" /> Connecting to Loadouts…
      </div>
    );
  }

  // The API is the single source of truth; without it there is nothing meaningful to show,
  // so we surface exactly how to fix it rather than silently faking data.
  if (session.error) {
    return (
      <div className="h-screen flex items-center justify-center bg-stone-950 p-8">
        <div className="max-w-md text-center">
          <h1 className="text-xl font-medium text-white mb-2">Can’t reach the Loadouts API</h1>
          <p className="text-sm text-stone-500 mb-4">{session.error}</p>
          <pre className="text-left text-xs bg-stone-900 border border-stone-800 rounded-lg p-4 text-stone-400 overflow-x-auto">
            cd loadouts/backend{'\n'}go run ./cmd/server
          </pre>
          <button
            onClick={session.reload}
            className="mt-4 px-4 py-2 rounded-lg bg-orange-500 hover:bg-orange-400 text-sm font-medium text-white"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-stone-950 font-sans overflow-hidden">
      <nav className="w-[72px] bg-stone-950 flex flex-col items-center py-3 border-r border-stone-900 shrink-0">
        <TabButton icon={<Home size={20} />} label="Home" active={!loadoutId && tab === 'home'} onClick={() => { setLoadoutId(null); setTab('home'); }} />
        <TabButton icon={<Compass size={20} />} label="Discover" active={!loadoutId && tab === 'discover'} onClick={() => { setLoadoutId(null); setTab('discover'); }} />
        <TabButton icon={<Users size={20} />} label="Communities" active={!loadoutId && tab === 'communities'} onClick={() => { setLoadoutId(null); setTab('communities'); }} />
        <TabButton icon={<Database size={20} />} label="Garage" active={!loadoutId && tab === 'garage'} onClick={() => { setLoadoutId(null); setTab('garage'); }} />

        <div className="flex-1" />

        {/* Profile switcher: one User can hold several Profiles, and everything you see
            (ownership, private notes, community roles) depends on which one is active. */}
        <div className="w-full px-2 pb-1">
          {session.profiles.map((p) => {
            const active = session.profile?.id === p.id;
            return (
              <button
                key={p.id}
                onClick={() => session.switchProfile(p.id)}
                title={`${p.display_name} (@${p.handle})`}
                className={`w-full aspect-square mb-2 rounded-2xl text-xs font-bold transition-all ${
                  active
                    ? 'bg-orange-500 text-white rounded-xl'
                    : 'bg-stone-800 text-stone-400 hover:bg-stone-700 hover:rounded-xl'
                }`}
              >
                {p.handle.slice(0, 2).toUpperCase()}
              </button>
            );
          })}
        </div>
      </nav>

      <div className="flex-1 flex overflow-hidden">
        {loadoutId ? (
          <LoadoutEditorView loadoutId={loadoutId} onBack={() => setLoadoutId(null)} />
        ) : (
          <>
            {tab === 'home' && <HomeView onOpenLoadout={openLoadout} />}
            {tab === 'discover' && <DiscoverView onOpenLoadout={openLoadout} />}
            {tab === 'communities' && <CommunitiesView onOpenLoadout={openLoadout} />}
            {tab === 'garage' && <GarageView />}
          </>
        )}
      </div>
    </div>
  );
}

function TabButton({
  icon,
  label,
  active,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button onClick={onClick} className="group relative flex items-center justify-center w-12 h-12 mb-2">
      <div
        className={`absolute left-0 w-1 bg-white rounded-r-full transition-all ${
          active ? 'h-8' : 'h-0 group-hover:h-4'
        }`}
      />
      <div
        className={`flex items-center justify-center w-12 h-12 rounded-[24px] transition-all ${
          active
            ? 'bg-orange-500 rounded-[16px] text-white'
            : 'bg-stone-800 text-stone-400 group-hover:bg-orange-500 group-hover:text-white group-hover:rounded-[16px]'
        }`}
      >
        {icon}
      </div>
      <div className="absolute left-16 bg-black text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 pointer-events-none whitespace-nowrap z-50">
        {label}
      </div>
    </button>
  );
}
