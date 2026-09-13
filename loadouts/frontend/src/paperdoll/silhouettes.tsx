import type { BodyPart, PropName } from './archetypes';

/**
 * The figure itself, ported from prototypes/rpg-inventory.html.
 *
 * Highlighting is driven by a Set of active parts rather than per-shape props, so a caller
 * only has to say "torso is lit" and every shape belonging to the torso responds. Colours
 * are inline styles rather than Tailwind classes on purpose: these are computed per render
 * from the active set, and a dynamic class name is exactly the thing a JIT Tailwind build
 * cannot see.
 */

const GOLD = '#fbbf24';

const IDLE = { fill: '#232936', stroke: '#39414f', strokeWidth: 1.5 } as const;
const LIT = { fill: GOLD, stroke: '#fde68a', strokeWidth: 1.5, filter: 'url(#pdGlow)' } as const;

export type ActiveParts = ReadonlySet<BodyPart | PropName>;

function useParts(active: ActiveParts) {
  const on = (p: BodyPart | PropName) => active.has(p);
  const fill = (p: BodyPart | PropName) => (on(p) ? LIT : IDLE);
  const line = (p: BodyPart | PropName, width: number) =>
    on(p)
      ? { stroke: GOLD, strokeWidth: width, filter: 'url(#pdGlow)', fill: 'none', strokeLinecap: 'round' as const }
      : { stroke: '#39414f', strokeWidth: width, fill: 'none', strokeLinecap: 'round' as const };
  return { on, fill, line };
}

/** Shared filter and floor gradient. Ids are prefixed to avoid colliding with plugin SVGs. */
function SvgDefs() {
  return (
    <defs>
      <filter id="pdGlow" x="-70%" y="-70%" width="240%" height="240%">
        <feGaussianBlur stdDeviation="4.5" result="blur" />
        <feMerge>
          <feMergeNode in="blur" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>
      <radialGradient id="pdFloor" cx="50%" cy="50%">
        <stop offset="0%" stopColor={GOLD} stopOpacity=".10" />
        <stop offset="100%" stopColor={GOLD} stopOpacity="0" />
      </radialGradient>
    </defs>
  );
}

/**
 * A loaded backpacker. Unlike the prototype the legs are a real part, because "Worn: Legs"
 * is a genuine slot in the seeded Basic Backpacking template and an inert limb under a
 * populated slot reads as a bug.
 */
export function HikerSilhouette({ active }: { active: ActiveParts }) {
  const { fill, line, on } = useParts(active);
  return (
    <svg viewBox="0 0 240 380" className="w-full h-auto max-h-[340px]" role="img" aria-label="Backpacker equipment silhouette">
      <SvgDefs />
      <ellipse cx="120" cy="348" rx="86" ry="16" fill="url(#pdFloor)" />

      {/* Pack sits behind the body. */}
      <g style={fill('back')}>
        <rect x="54" y="72" width="132" height="124" rx="24" />
        <rect x="70" y="96" width="100" height="30" rx="9" opacity=".38" />
        <rect x="70" y="140" width="100" height="22" rx="8" opacity=".26" />
      </g>

      <g style={fill('legs')}>
        <path d="M99 202 q-5 56 -8 108 l22 0 q3 -54 7 -108 z" />
        <path d="M141 202 q5 56 8 108 l-22 0 q-3 -54 -7 -108 z" />
      </g>

      <g style={fill('torso')}>
        <path d="M88 92 L152 92 Q162 92 162 104 L155 196 Q154 208 142 208 L98 208 Q86 208 85 196 L78 104 Q78 92 88 92 Z" />
      </g>

      <g style={fill('head')}>
        <circle cx="120" cy="56" r="27" />
        <rect x="83" y="42" width="74" height="10" rx="5" />
      </g>

      <g style={fill('hands')}>
        <circle cx="62" cy="180" r="11" />
        <circle cx="178" cy="180" r="11" />
      </g>
      <g style={line('hands', 5)}>
        <path d="M84 110 Q66 142 62 172" />
        <path d="M156 110 Q174 142 178 172" />
      </g>
      {/* Trekking poles, dimmed until the hands slot is live. */}
      <g style={line('hands', 3.5)} opacity={on('hands') ? 1 : 0.55}>
        <path d="M58 158 L48 332" />
        <path d="M182 158 L192 332" />
      </g>

      <g style={fill('feet')}>
        <path d="M92 306 q-18 0 -18 14 l0 10 q0 7 7 7 l36 0 q7 0 7 -7 l0 -24 z" />
        <path d="M148 306 q18 0 18 14 l0 10 q0 7 -7 7 l-36 0 q-7 0 -7 -7 l0 -24 z" />
      </g>
    </svg>
  );
}

/**
 * The fallback figure: no pack, no poles, just a body. Used by any template without an
 * explicit silhouette, which is most of them and all user-created ones. Falling back to
 * "no paperdoll" would be worse than the flat grid this replaces.
 */
export function HumanoidSilhouette({ active }: { active: ActiveParts }) {
  const { fill, line } = useParts(active);
  return (
    <svg viewBox="0 0 240 380" className="w-full h-auto max-h-[340px]" role="img" aria-label="Equipment silhouette">
      <SvgDefs />
      <ellipse cx="120" cy="348" rx="78" ry="15" fill="url(#pdFloor)" />

      <g style={fill('legs')}>
        <path d="M101 204 q-6 56 -9 106 l22 0 q4 -52 8 -106 z" />
        <path d="M139 204 q6 56 9 106 l-22 0 q-4 -52 -8 -106 z" />
      </g>

      <g style={fill('torso')}>
        <path d="M90 94 L150 94 Q160 94 160 106 L153 198 Q152 210 140 210 L100 210 Q88 210 87 198 L80 106 Q80 94 90 94 Z" />
      </g>

      <g style={fill('head')}>
        <circle cx="120" cy="58" r="26" />
      </g>

      <g style={fill('hands')}>
        <circle cx="66" cy="182" r="10" />
        <circle cx="174" cy="182" r="10" />
      </g>
      <g style={line('hands', 5)}>
        <path d="M86 112 Q70 144 66 174" />
        <path d="M154 112 Q170 144 174 174" />
      </g>

      {/* No pack on the generic figure, so `back` gets a carried-bag hint instead. */}
      <g style={fill('back')} opacity=".9">
        <rect x="96" y="120" width="48" height="56" rx="10" opacity=".5" />
      </g>

      <g style={fill('feet')}>
        <path d="M94 306 q-17 0 -17 13 l0 10 q0 7 7 7 l34 0 q7 0 7 -7 l0 -23 z" />
        <path d="M146 306 q17 0 17 13 l0 10 q0 7 -7 7 l-34 0 q-7 0 -7 -7 l0 -23 z" />
      </g>
    </svg>
  );
}

/**
 * Template props: things that are carried but not worn, drawn beside the figure rather
 * than on it. A tent is pitched, so highlighting a limb for it would be a lie.
 */
export function TentProp({ active }: { active: ActiveParts }) {
  const { fill } = useParts(active);
  return (
    <svg viewBox="0 0 120 80" className="w-full h-auto" role="img" aria-label="Shelter">
      <g style={fill('tent')}>
        <path d="M60 10 L104 68 L16 68 Z" />
        <path d="M60 10 L60 68" opacity=".35" />
        <path d="M46 68 L60 34 L74 68 Z" opacity=".45" />
      </g>
    </svg>
  );
}

export type SilhouetteComponent = (props: { active: ActiveParts }) => JSX.Element;

/**
 * Templates that get a figure of their own. Everything else uses the humanoid.
 *
 * A small declared override table, keyed on template name because template ids are
 * generated fresh on every in-memory reseed. Deliberately not inference: a template is
 * either listed here or it gets the generic figure.
 */
const SILHOUETTE_BY_TEMPLATE: Record<string, SilhouetteComponent> = {
  'Basic Backpacking': HikerSilhouette,
};

export function silhouetteFor(templateName: string): SilhouetteComponent {
  return SILHOUETTE_BY_TEMPLATE[templateName] ?? HumanoidSilhouette;
}
