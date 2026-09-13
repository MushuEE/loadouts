# Porting the prototype into the real app

The prototype ([`prototypes/rpg-inventory.html`](prototypes/rpg-inventory.html), branch
`design/rpg-inventory-prototype`) is the design target. The React app has the real data.
This plan moves the second toward the first, starting with the paperdoll.

Incremental: every phase ships a working app.

---

## The one real modelling gap: nothing is "worn"

The paperdoll's whole premise is that **some gear is on your body and the rest is in your
pack**. The prototype encodes this directly — each item has `worn: true` and a slot id that
names a body part (`torso`, `feet`, `back`).

The real model has neither. A `SlotDefinition` is `{id, name, accepted_categories,
required, max_items, position}`. Nothing says where a slot lives on a body, or whether it
is on the body at all. The seeded *Basic Backpacking* template only hints at it by naming
slots `"Worn: Torso"` / `"Worn: Legs"` / `"Worn: Feet"`.

So a faithful port needs slots to carry presentation intent. Three ways:

| | How | Cost | Breaks on |
| --- | --- | --- | --- |
| **A. Frontend registry** | Map template id → silhouette + per-slot body part | None. No backend change | Any template not in the map, i.e. every user-created one |
| **B. Slot fields** | Add `paperdoll_part` / `worn` / `column` to `SlotDefinition` | Schema + template version migration | Nothing, but it is the slow path |
| **C. Heuristics** | Infer from slot id and category | None | Silently, and confusingly |

**Recommendation: A now, B as the real answer, C never.** The registry is a deliberate
stopgap that keeps the schema decision open until the paperdoll has earned it. Phase 1
ships A behind a single module so that swapping in B later touches one file.

> [!IMPORTANT]
> A generic humanoid fallback is not optional. Freeform loadouts and user templates have no
> registry entry, and "no paperdoll at all" would be a visible regression from today's grid.
> Unmapped slots render in a neutral column beside the figure and highlight nothing.

---

## Phase 0 — Unblock the editor (do first, it is small)

Today the app looks broken for a reason that has nothing to do with the paperdoll: the
default profile is `fitcheck`, who owns one of the three seeded loadouts, and for the other
two the `+` buttons simply **vanish with no explanation**.

- [ ] Read-only banner: *"Viewing as @fitcheck — you don't own this loadout"* with a Fork CTA
- [ ] **Fix `fork()` in `LoadoutEditorView`**, which sets `window.location.hash` that nothing
      reads, so forking from the editor silently does nothing. `DiscoverView.fork()` is the
      correct reference. This is the escape hatch from the read-only state, so it has to
      work before the banner is worth anything
- [ ] Distinguish *slot is full* from *you cannot edit*. They look identical today, and
      `max_items: 0` means "exactly 1", so a filled single slot legitimately hides its `+`

Phase 0 is what actually makes the app explorable. Without it the paperdoll is a prettier
dead end.

---

## Phase 1 — Paperdoll

- [ ] `src/paperdoll/parts.tsx` — `useParts`, `SvgDefs`, the gold-glow filter and floor
      gradient. Straight port
- [ ] `src/paperdoll/silhouettes.tsx` — `Hiker`, `Bike`, `Runner`, plus a generic `Humanoid`
      fallback. Straight port, plus **`legs` as a real part** (the hiker's legs are currently
      hardcoded inert, but `Worn: Legs` is a real slot in the seeded template)
- [ ] `src/paperdoll/registry.ts` — the option-A map: template id → `{silhouette, slots: {slotId → {part, column}}}`.
      One file, so option B replaces exactly this
- [ ] `src/components/Paperdoll.tsx` — `PaperdollSlot` + `Paperdoll`, adapted from
      `ResolvedEntry[]` instead of the prototype's flat item array
- [ ] Wire into `LoadoutEditorView` above the existing slot grid; unmapped slots keep using
      the grid, so nothing disappears

Deliberately **not** in phase 1: rarity tiers and the encumbrance gauge. Rarity has no
backing field, and inventing one in the frontend would be the same mistake as heuristic
slot mapping.

---

## Phase 2 — Bag grid

- [ ] Replace the flat 3-column grid with the prototype's square bag cells for everything
      not on the paperdoll
- [ ] Selection state shared with the paperdoll: one selected entry, highlighted in both

## Phase 3 — Inspector

- [ ] Port the three-layer inspector. This one is a genuine upgrade over the prototype,
      because the layers are **real** here: `core.ResolveLayers` already returns a
      `provenance` map keyed `"namespace.key" → layer`, which the prototype faked

## Phase 4 — Theme

- [ ] Move from the current stone/orange palette to the prototype's `#0d0f12` / `#15181e` /
      `#1b1f27` with Inter + JetBrains Mono. Last, because it touches every view and is the
      easiest thing to redo

---

## Open questions

1. **Does the prototype branch get merged?** It is 3 commits, unpushed. It could stay a
   local reference, or land under `prototypes/` as a design artifact. It should not become
   a second app.
2. **When does the registry become schema?** Suggested trigger: the first user-created
   template that wants a paperdoll. Until then the map is honest.
3. **Rarity.** The prototype's five tiers carry real visual weight. There is no such field.
   Is it a genuine product concept (cost? weight percentile? community rating?) or just
   prototype decoration?
