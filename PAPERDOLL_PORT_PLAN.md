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

So slots have to carry presentation intent. Three ways:

| | How | Cost | Breaks on |
| --- | --- | --- | --- |
| **A. Archetype defaults** | A slot resolves to a body part by its archetype — shirt→torso, pants→legs, hat→head, backpack→back | None. No backend change | Nothing visibly; unmapped slots fall to a neutral column |
| **B. Slot fields** | Add `paperdoll_part` to `SlotDefinition` | Schema + template version migration | Nothing, but it is the slow path |
| **C. Silent inference** | Guess from arbitrary slot ids and free-text names | None | Silently, and confusingly |

**Decision: A now, B as the eventual override, C never.**

A is not the same as C, and the difference is the whole point. A is a **published vocabulary
of archetypes with a declared default part each**, plus a visible fallback for anything
outside it. C is pattern-matching on strings and hoping. A slot says *"I am a `hat` slot"*
and the mapping is a lookup anyone can read; it does not try to divine that
`"Worn: Head (winter)"` probably means a head.

> [!IMPORTANT]
> Defaults must be **overridable, never mandatory**. When B lands, a template sets
> `paperdoll_part` explicitly and the archetype default is just what it starts at. Today's
> seeded slots already imply their archetypes (`worn-torso`, `worn-legs`, `worn-feet`,
> `pack`), so the defaults cover the real templates on day one.

Unmapped slots — Freeform, anything exotic — render in a neutral column flanking the figure
and highlight nothing. They never disappear.

### Not every slot is a body part

A tent is not worn. Some slots belong to the **template**, not the body: a shelter gets its
own SVG pitched beside the figure rather than a highlight on a limb. So the paperdoll has
two kinds of target:

- **Body parts** — head, torso, legs, feet, hands, back
- **Template props** — tent, bike, and similar, each its own SVG, owned by the template
  rather than the archetype vocabulary

The prototype already proves out the second: the cycling silhouette *is* a prop, with the
rider implied.

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

**Layout: gear surrounds the figure, WoW-style.** Slot frames flank the silhouette in left
and right columns, figure centred. Not a list beside a picture — the figure is the middle
of the composition.

- [ ] `src/paperdoll/parts.tsx` — `useParts`, `SvgDefs`, the gold-glow filter and floor
      gradient. Straight port
- [ ] `src/paperdoll/silhouettes.tsx` — `Hiker`, `Bike`, `Runner`, plus a generic `Humanoid`
      fallback. Straight port, plus **`legs` as a real part** (the hiker's legs are currently
      hardcoded inert, but `Worn: Legs` is a real slot in the seeded template)
- [ ] `src/paperdoll/archetypes.ts` — the published vocabulary: archetype → default body
      part. One file, so per-template overrides later slot in behind it
- [ ] `src/paperdoll/props.tsx` — template-owned SVGs that are not body parts. A tent
      pitched beside the figure is the first one
- [ ] `src/components/Paperdoll.tsx` — `PaperdollSlot` + `Paperdoll`, driven by
      `ResolvedEntry[]` instead of the prototype's flat item array
- [ ] Wire into `LoadoutEditorView`; unmapped slots keep the existing grid so nothing
      disappears

### Interaction

- [ ] **Hover a populated slot → its doll element highlights.** Hover is the primary
      binding, not selection. It is exploratory: sweep the slots and watch the figure light
      up, no clicking and no state to undo
- [ ] **Empty slots still render.** A slot is a statement about what the template *expects*,
      so an unfilled one is information — it shows a gap in the kit. Rendering only filled
      slots would hide exactly the thing a gear list is for
- [ ] Hovering an empty slot highlights nothing. There is no item to point at, and a lit
      limb with nothing in it reads as a bug
- [ ] Multi-item slots light their part once, as a unit

> [!NOTE]
> The prototype keys highlighting off *selection*, because it also drives an inspector
> panel. Hover and selection can coexist — hover previews, click pins — but hover is what
> makes the figure feel alive, so it ships first and selection follows in Phase 3 with the
> inspector.

Deliberately **not** in phase 1: rarity tiers and the encumbrance gauge. Rarity has no
backing field, and inventing one in the frontend would be the same mistake as silent slot
inference.

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
