# Community Favorites

A community endorses a loadout it does not own. A profile bookmarks one it wants to keep.
Same mechanism, two scopes.

## 1. Why this instead of community-owned loadouts

The recursive-templates work needed an answer to *"how does a community offer a canonical
meal kit that every member's trip can use?"* The first answer was ownership: give
`Loadout` the `OwnerType`/`OwnerID` pair that `Template` has, let a community own a
loadout, and let its admins edit it.

That was built and then removed. See the rejected-design note in
`RECURSIVE_TEMPLATES_PLAN.md` for the full autopsy; the short version is that shared
ownership means **a mod's edit silently changes the weight of trips planned months ago**,
and a packing list whose numbers move on their own is worse than no shared kit at all.

The real need was never ownership. It was *discovery* — a durable, visible signal that says
"this is the good one". That is a favorite:

| | Community ownership | Community favorite |
| --- | --- | --- |
| Who can edit the kit | Any admin | Only its owner |
| Effect on a member's trip | Changes under them | None, ever |
| How a member uses it | Attach the shared one | Fork it, then attach their copy |
| Cost to undo | Ownership transfer | Delete a row |
| New authority rules | Three-way check everywhere | One check, at the point of endorsement |

## 2. Decisions

### A favorite is a live pointer carrying a fingerprint

The favorite points at the loadout as it is *now*. It does not freeze a copy.

Freezing was considered and rejected: real loadout versioning is a bigger concept than the
ownership code this replaced, and it creates a second source of truth that immediately
starts drifting from the live loadout, with "an update is available" UX to reconcile them.

But a bare live pointer has a real failure: **endorsement hijack**. A mod endorses an
excellent budget kit; the owner later swaps every item for sponsored junk and renames it;
the community is still pointing at it, with the community's credibility attached.

So the favorite records a **fingerprint** of the loadout's substance at the moment it was
confirmed. Nothing is frozen and nothing blocks the owner — but the endorsement can be
shown as *stale*, and a mod can re-confirm it in one click. Cheap to compute, one text
column, and it turns a silent hijack into a visible diff.

```
fingerprint = fp2:sha256( name ‖ sorted( slot | item | child | quantity | selected ) )
```

Sorted by entry ID so the hash is stable; **position is excluded** because reordering cards
in the UI is cosmetic and should not cry wolf. Description and cover image are excluded for
the same reason. Name *is* included — "UL Budget Kit" becoming "Sponsored Kit" is exactly
the hijack we are trying to surface.

The digest is prefixed with its algorithm so it can change later without anyone having to
guess how an old value was computed. It shipped as `fp1:` and became `fp2:` when
sub-loadouts arrived and `child` and `selected` joined the inputs; the bump makes every
older endorsement read as stale exactly once, which is the honest answer, since those
digests genuinely could not see a whole category of change.

> [!NOTE]
> The fingerprint is deliberately **local**: it covers this loadout's own name and entry
> rows, and does not follow a sub-loadout reference into the child's contents. Editing
> "Meals Day 1" changes *that* loadout's fingerprint, so an endorsement of the meal kit
> goes stale correctly — but an endorsement of a trip referencing it does not. Chasing the
> whole subtree would make every parent's freshness depend on strangers' edits, which is a
> much noisier signal and wants its own decision.

### Two scopes, one table

A favorite is scoped to a **profile** (a private bookmark) or a **community** (a public
endorsement). It is the same row shape, so it is one table with `scope_type`/`scope_id`.

This is the same polymorphic pair that was just rejected for ownership, and the difference
is worth stating: **a favorite confers no authority.** Nothing reads it to decide whether
someone may edit something. It never has to be consulted on a mutation, so it cannot grow
the three-way checks that sank community ownership. It is an index, not a permission.

### Endorsing requires authority over the scope, not over the loadout

| Scope | Who may favorite | Extra rule |
| --- | --- | --- |
| `profile` | That profile, and only that profile | The loadout must be visible to you |
| `community` | An admin of that community | The loadout must not be private |

The private rule is specific to community scope: endorsing something the members cannot
open is a dead link that also advertises the existence of a private loadout. A personal
bookmark of your own private draft is fine and useful.

Note what is *not* required: permission from the loadout's owner. Endorsement is speech
about a public object, and the owner already controls the object itself — they can make it
private or delete it, and the endorsement degrades accordingly.

### A favorite outlives its target

If the loadout is deleted, or the owner makes it private, the favorite row stays and the
listing reports it as unavailable rather than quietly vanishing.

This matches the `child_loadout_id` decision in the recursive work: a dangling reference is
a state worth showing. A community that endorsed six kits and now sees five should be told
why, not silently shown five.

## 3. Data model

```go
type FavoriteScope string // "profile" | "community"

type Favorite struct {
    ID             string
    ScopeType      FavoriteScope
    ScopeID        string // profile or community ID
    LoadoutID      string
    Note           string // why this one is the good one
    Fingerprint    string // of the loadout when last confirmed
    ActorProfileID string // who endorsed, or last re-confirmed
    CreatedAt      time.Time
    ConfirmedAt    time.Time
}
```

A read view adds the two things only the service can know:

```go
type FavoriteView struct {
    Favorite
    Available bool                // the loadout still exists and is visible to the viewer
    Stale     bool                // its substance changed since ConfirmedAt
    Loadout   *core.LoadoutSummary // nil when unavailable
}
```

### Migration `0008_community_favorites.sql`

```sql
CREATE TABLE favorites (
    id              TEXT PRIMARY KEY,
    scope_type      TEXT NOT NULL CHECK (scope_type IN ('profile', 'community')),
    scope_id        TEXT NOT NULL,
    loadout_id      TEXT NOT NULL,
    note            TEXT NOT NULL DEFAULT '',
    fingerprint     TEXT NOT NULL DEFAULT '',
    actor_profile_id TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    confirmed_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_favorites_unique ON favorites(scope_type, scope_id, loadout_id);
CREATE INDEX idx_favorites_loadout ON favorites(loadout_id);
```

No foreign key on `loadout_id`, for the reason in §2: the row must outlive the loadout.

## 4. API

| Method | Path | Meaning |
| --- | --- | --- |
| `PUT` | `/loadouts/{id}/favorite` | Favorite, or re-confirm an existing one (idempotent) |
| `DELETE` | `/loadouts/{id}/favorite?scope_type=&scope_id=` | Un-favorite |
| `GET` | `/loadouts/{id}/favorites` | Who endorsed this loadout |
| `GET` | `/favorites?scope_type=&scope_id=` | A scope's shelf, with `stale` and `available` flags |

`PUT` rather than `POST` because favoriting twice must not create two rows, and because
re-confirming a stale endorsement is the same operation as making it.

## 5. Work breakdown

| Phase | Scope | Status |
| --- | --- | --- |
| 1 | Core: `Favorite`, `FavoriteScope`, `FingerprintLoadout`, unit tests | |
| 2 | Store: five methods, migration `0008`, memory + Postgres | |
| 3 | Service: authority, staleness, availability, `LoadoutService.summarize` extraction | |
| 4 | HTTP API + handler wiring | |
| 5 | Frontend: endorse button, community shelf, "edited since endorsed" badge | |
| 6 | Seed + docs + `smoke_favorites.sh` | |

## 6. Non-goals

| Deferred | Why |
| --- | --- |
| Recursive fingerprints that follow sub-loadouts into their contents | Would make a trip's endorsement go stale whenever a stranger edits a referenced kit. A much noisier signal that wants its own decision, not a silent extension of this one |
| A denormalized `favorite_count` on `Loadout` | A count query is fine at this size, and a cached counter is a consistency bug waiting to happen |
| Frozen snapshots / loadout versioning | Bigger than the ownership model it would replace; the fingerprint covers the actual risk |
| Notifying the owner that a community endorsed them | Wants a notification system, which does not exist yet |
| Ranking or weighting endorsements in discovery | Endorsement is the signal; what discovery does with it is a separate feature |
| Moderation of endorsements by the loadout's owner | The owner controls the object; if they object to an endorsement, unpublishing is the lever |
