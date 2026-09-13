import type { SlotDefinition } from '../api/types';

/**
 * Where a slot lives on the paperdoll.
 *
 * A `SlotDefinition` says what a slot accepts, not where it sits on a body, so the
 * paperdoll has to supply that. This module is the published vocabulary that does it.
 *
 * It keys off **accepted categories**, not slot ids, and that choice matters. Slot ids are
 * per-template and arbitrary: the same chest slot is `worn-torso` in Basic Backpacking and
 * `top` in Daily Fit. Categories are the shared vocabulary that items are actually filed
 * under, so a mapping built on them works for templates nobody has written yet.
 *
 * This is a lookup table, not inference. It does not try to divine meaning from free text
 * like "Worn: Head (winter)". A category is either in the table or the slot is unmapped,
 * and unmapped slots are rendered plainly rather than guessed at.
 */

/** A highlightable region of the figure. */
export type BodyPart = 'head' | 'torso' | 'legs' | 'feet' | 'hands' | 'back';

/** A template-owned object that is not worn. A tent is pitched, not equipped. */
export type PropName = 'tent';

export type PaperdollTarget =
  | { kind: 'body'; part: BodyPart }
  | { kind: 'prop'; prop: PropName }
  | { kind: 'unmapped' };

export const UNMAPPED: PaperdollTarget = { kind: 'unmapped' };

/**
 * Category to paperdoll target. Extend this rather than special-casing at a call site.
 *
 * Categories deliberately absent - kitchen, fuel, consumable, electronics, organizer - are
 * things that ride inside the pack rather than on the body. They could all be pointed at
 * `back`, but then hovering a stove would light the same region as hovering the backpack,
 * which says something misleading about where the gear is. Until there is a distinct
 * "carried" treatment they stay unmapped, which is honest.
 */
const CATEGORY_TARGETS: Record<string, PaperdollTarget> = {
  // Worn on the body.
  shirt: { kind: 'body', part: 'torso' },
  outerwear: { kind: 'body', part: 'torso' },
  pants: { kind: 'body', part: 'legs' },
  shoes: { kind: 'body', part: 'feet' },
  poles: { kind: 'body', part: 'hands' },
  pack: { kind: 'body', part: 'back' },

  // Head coverings. No seeded template uses these yet, but they are the archetypes most
  // likely to arrive next and the multi-select headgear slot in the prototype is built on
  // them, so the vocabulary carries them from the start.
  hat: { kind: 'body', part: 'head' },
  headgear: { kind: 'body', part: 'head' },
  helmet: { kind: 'body', part: 'head' },

  // Not worn at all.
  shelter: { kind: 'prop', prop: 'tent' },
};

/**
 * Resolve a slot to the thing it should highlight.
 *
 * A slot may accept several categories - "Worn: Torso" takes both `shirt` and `outerwear`.
 * They agree here, and where they ever disagree the first mapped category wins, which is a
 * stable rule rather than an arbitrary one because `accepted_categories` is ordered by the
 * template author.
 */
export function targetForSlot(slot: SlotDefinition): PaperdollTarget {
  for (const category of slot.accepted_categories ?? []) {
    const target = CATEGORY_TARGETS[category];
    if (target) return target;
  }
  return UNMAPPED;
}

export function isMapped(target: PaperdollTarget): boolean {
  return target.kind !== 'unmapped';
}

/**
 * Which column a slot hangs in, WoW-style: gear frames flank the figure rather than
 * listing beside it. Deterministic, so a slot does not move between renders.
 */
export function columnForTarget(target: PaperdollTarget): 'left' | 'right' {
  if (target.kind === 'body') {
    // Head and torso lead the left column because that is where the eye starts; the
    // extremities and the pack balance the right.
    return target.part === 'head' || target.part === 'torso' || target.part === 'legs'
      ? 'left'
      : 'right';
  }
  return 'right';
}
