import { Globe, User } from 'lucide-react';
import { useGearScope } from '../lib/gearScope';

/**
 * The [My gear | Everyone] switch. Every copy shares one app-wide setting, so it reads the
 * same wherever you meet it.
 */
export function ScopeToggle({ size = 'md' }: { size?: 'sm' | 'md' }) {
  const [scope, setScope] = useGearScope();
  const pad = size === 'sm' ? 'px-2.5 py-1 text-[11px]' : 'px-3.5 py-1.5 text-sm';
  const icon = size === 'sm' ? 'w-3 h-3' : 'w-3.5 h-3.5';
  const option = (value: typeof scope, label: string, Icon: typeof User, title: string) => (
    <button
      role="radio"
      aria-checked={scope === value}
      onClick={() => setScope(value)}
      title={title}
      className={`${pad} flex items-center gap-1.5 rounded-md font-medium transition-colors ${
        scope === value ? 'bg-orange-600 text-white shadow' : 'text-stone-400 hover:text-white'
      }`}
    >
      <Icon className={icon} />
      {label}
    </button>
  );
  return (
    <div role="radiogroup" aria-label="Search scope" className="inline-flex p-0.5 rounded-lg bg-stone-900 border border-stone-800 shrink-0">
      {option('mine', 'My gear', User, 'Gear you use, tagged, annotated or imported, matched against your own tags')}
      {option('everyone', 'Everyone', Globe, "The whole catalogue, matched against anyone's tags")}
    </div>
  );
}
