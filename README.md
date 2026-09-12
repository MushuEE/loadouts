# Loadouts

Curating, tracking, sharing, and extending the paraphernalia that represents your hobby is
hard today. **Loadouts** is a platform for people and communities to build, tailor, and share
both *templates* and populated *loadouts* — backpacking kits, streetwear fits, MMO gear,
cycling setups, whatever you're into.

This repository contains a Day 0 MVP: every concept in the object model exists end to end
(model → store → service → API → UI) at the simplest fidelity that is still correct.

---

## Object model

| Concept | What it is |
| --- | --- |
| **User** | Account root. Owns 1..n Profiles. Authors nothing directly. |
| **Profile** | The public persona that owns loadouts, templates, and community memberships. One human can keep a backpacking profile and a streetwear profile. |
| **Item** | A global product (physical or virtual). Public and effectively immutable — everything hobby-specific lives in the layers above it. |
| **Community** | A group layer above Profiles (UL Backpacking, NYC Fashion, WoW). Hosts templates, loadouts, and its own item metadata layer. |
| **Template** | The scaffolding for a loadout: a versioned list of slots. `platform-freeform` imposes no structure at all. |
| **Loadout** | The marriage of a Template and Items. The core entity people share, browse, edit, and fork. |

### The four metadata layers

An item is resolved in a *view context* (who is looking, and where):

```
1. Global base        items.base_metadata                    public, shared
2. Community layer    community_item_layers.metadata         public, only inside that community
3. User public layer  profile_item_layers.public_metadata    public, attached to that profile
4. User private layer profile_item_layers.private_metadata   owner-only, never served to others
```

Layers deep-merge left to right, and every response carries a `provenance` map
(`"ul_backpacking.ul_score" -> "community"`) so the UI can explain where each value came from.
The private layer is redacted in the service, not the handler, so it cannot leak through a
new endpoint by accident.

The `core` namespace (`weight_g`, `cost_cents`, `consumable`) is the one thing the platform
itself understands; it powers base weight, total weight, and cost rollups for any hobby.

---

## Running it

### Backend (Go)

```bash
cd loadouts/backend
go run ./cmd/server        # in-memory store, auto-seeded demo data, :8080
```

With Postgres:

```bash
cd loadouts
docker compose up -d
# apply backend/migrations/*.sql in order, then:
DATABASE_URL="postgres://..." SEED=1 go run ./cmd/server
```

### Frontend (React + Vite + Tailwind)

```bash
cd loadouts/frontend
npm install
npm run dev            # http://localhost:5173
```

The frontend talks to `http://localhost:8080/api/v1` by default; override with `VITE_API_URL`.

### Demo data

Booting with the in-memory store seeds: two users, three profiles
(`@gearhead`, `@fitcheck`, `@trailsponsor`), two communities (`ul-backpacking`, `nyc-fashion`)
including the UL community's `{ul_score, comfort, durability}` item layer, a community
template plus a profile template, ~20 real gear items, and three published loadouts.

---

## Day 0 auth

There are no passwords yet. The client asserts which profile it is acting as with the
`X-Profile-ID` header (ID or `@handle`), resolved in [`internal/auth`](loadouts/backend/internal/auth/auth.go).
That is the single swap point for real auth. In the UI, the profile switcher in the left rail
*is* the login screen.

---

## API surface

```
GET    /healthz

POST   /api/v1/users                       GET /api/v1/users/{id}/profiles
POST   /api/v1/profiles                    GET /api/v1/profiles/me
GET    /api/v1/profiles/{handle}           GET /api/v1/profiles/{handle}/loadouts
                                           GET /api/v1/profiles/{handle}/communities

GET    /api/v1/communities                 POST /api/v1/communities
GET    /api/v1/communities/{slug}          POST /api/v1/communities/{slug}/join|leave
GET    /api/v1/communities/{slug}/members|templates|loadouts
PUT    /api/v1/communities/{slug}/items/{itemID}/layer     # admin only

GET    /api/v1/templates                   POST /api/v1/templates
GET    /api/v1/templates/{id}              GET/POST /api/v1/templates/{id}/versions

GET    /api/v1/loadouts                    POST /api/v1/loadouts
GET    /api/v1/loadouts/{id}               PATCH/DELETE /api/v1/loadouts/{id}
PUT    /api/v1/loadouts/{id}/entries
POST   /api/v1/loadouts/{id}/publish       POST /api/v1/loadouts/{id}/fork
GET    /api/v1/discover

PUT    /api/v1/loadouts/{id}/favorite      # endorse or bookmark (idempotent)
DELETE /api/v1/loadouts/{id}/favorite      # withdraw
GET    /api/v1/loadouts/{id}/favorite      # who vouches for this (communities only)
GET    /api/v1/favorites?scope_type=&scope_id=   # a community's shelf, or your bookmarks

GET    /api/v1/items                       POST /api/v1/items
GET    /api/v1/items/{id}?community=&owner=
GET/PUT /api/v1/items/{id}/layers/profile
GET    /api/v1/schemas                     POST /api/v1/schemas

GET    /api/v1/imports/suppliers           # retailers we have URL rules for
POST   /api/v1/imports/preview             # inspect a product URL, writes nothing
POST   /api/v1/imports/commit              # create the item from the confirmed draft

GET    /api/v1/plugins                     # the directory (filterable by surface)
POST   /api/v1/plugins                     # publish a plugin and its v1
GET    /api/v1/plugins/{id}                # detail, optionally at ?version=N
GET    /api/v1/plugins/{id}/versions       # version history
POST   /api/v1/plugins/{id}/versions       # publish a new version (author only)
PATCH  /api/v1/plugins/{id}/visibility     # list or unlist (author only)
GET    /api/v1/plugins/render              # evaluated views for one surface
GET    /api/v1/plugins/installs            # what is installed in a scope
POST   /api/v1/plugins/installs            # install, granting capabilities explicitly
PATCH  /api/v1/plugins/installs/{id}       # enable/disable, or replace settings
DELETE /api/v1/plugins/installs/{id}       # uninstall
GET    /api/v1/plugins/{id}/data           # the plugin's own namespaced storage
PUT    /api/v1/plugins/{id}/data/{key}     # written by the frame, via the parent
DELETE /api/v1/plugins/{id}/data/{key}
GET    /sandbox/plugins/{id}/versions/{v}/views/{view}/frame   # the embed frame itself
```

---

## Importing gear from a store

Paste a product link from REI, Amazon, Backcountry, Patagonia, Garage Grown Gear — or any
other store — and it becomes a catalog item.

```
URL ──▶ canonicalize ──▶ fetch ──▶ extract ──▶ Draft ──▶ [user edits] ──▶ commit
        (supplier +      (SSRF-    (JSON-LD →   (name, weight,          (Item +
         product ID)      guarded)  OG tags →    price, image,           ItemSource)
                                    title)       category guess)
```

A few properties worth knowing:

- **Canonicalization never touches the network.** `rei.com/product/894303/slug?utm_source=x`
  resolves to `(rei, 894303)` by string parsing alone. That gives us dedupe and a correct
  affiliate link even when the scrape fails.
- **A blocked retailer is a degraded success, not an error.** Amazon returns 503/403 to
  datacenter traffic. The import falls back to `status: "manual"` — we still know the ASIN
  and the outbound link, the user just fills in name/weight/price themselves.
- **Weights are normalized to grams** (`"2 lb 3 oz"` → 992.23g) because `core.weight_g` is
  what the loadout stats engine runs on. If a weight can't be found the field is left
  *blank*, never prefilled with 0, so a missing weight can't silently corrupt a loadout.
- **Re-importing is idempotent.** The same product from a share link, a search result, and
  an affiliate link all collapse to one catalog entry via `UNIQUE(supplier_id, product_id)`.
- **Imports land in the global catalog flagged `origin=import, verified=false`**, so they
  are immediately useful to everyone but still distinguishable from hand-curated entries.
- **Unknown stores still work**, resolving to the generic `other` supplier whose affiliate
  template is a passthrough to the original URL.

Adding a retailer means appending one entry to the registry in
[`internal/importer/url.go`](loadouts/backend/internal/importer/url.go); the extraction
stage is generic across all of them.

> [!NOTE]
> Fetching a user-supplied URL server-side is a classic SSRF sink. The guard is applied at
> dial time via `Dialer.Control`, which covers the original host, every redirect hop, and
> DNS rebinding in one place, plus a hostname check in `Canonicalize` so the commit path
> (which never fetches) is protected too.

---

## Communities point at good loadouts without owning them

A community can **endorse** a loadout, and a profile can **bookmark** one. Both are the
same row with a different scope, and neither gives anyone any control over the loadout.

This was the second answer to "how does a community offer a canonical meal kit?". The
first was to let communities *own* loadouts, which was built and then removed: shared
ownership means a moderator's edit silently changes the weight of trips that were planned
months ago. A member who wants a community's kit forks it and gets a stable copy of their
own — which is what you actually want from a packing list.

Endorsement takes an admin of the community, and the loadout must not be private. It
never needs the loadout owner's permission: endorsing is speech about a public object,
and the owner keeps the only lever that matters, which is control of the object.

**Endorsements can go stale.** Each one records a fingerprint of the loadout's name and
contents when it was confirmed, so if the owner later swaps the gear or renames it, the
endorsement is flagged *"edited since endorsed"* with a one-click re-confirm rather than
quietly vouching for something else. Cosmetic edits — description, cover image, dragging
cards around the grid — deliberately do not trip it.

Endorsements also outlive their target. Delete a loadout and the endorsement stays,
marked unavailable, because a community that endorsed six kits and now sees five should
be told why.

---

## Extending the UI with plugins

Every hobby measures itself differently, and we cannot ship a feature for each one. Users
and communities can add their own views — charts, tables, calculators, maps — and install
them onto surfaces in the app.

Authors are untrusted, so there are two tiers:

```
Widget ──▶ JSON spec ──▶ evaluated server-side ──▶ browser receives values only
Embed  ──▶ author HTML ──▶ sandboxed iframe ──▶ postMessage ──▶ authenticated parent
```

- **Widgets ship data, not code.** A spec says what to read and what expressions produce
  each number; the host evaluates it in Go and hands the frontend literal values. A bad
  expression is a publish-time `400`, never a broken page.
- **Embeds run the author's own HTML** in an iframe sandboxed with `allow-scripts` but
  deliberately *without* `allow-same-origin`. The opaque origin has no credentials, so the
  frame cannot call the API at all — everything privileged is relayed by the parent.
- **Capabilities are default-deny.** A manifest declares what it needs, an install records
  what was granted, and rendering intersects the two. A partial grant is refused outright.
- **Installs pin a version**, so publishing v2 cannot change behaviour someone already
  approved. A version that asks for more access waits for a fresh grant.
- **One broken plugin is one broken card**, not a failed request.

Three examples ship with the demo seed: a pack-weight pie chart, a table over a
community's own `ul_score` metadata layer, and a Google Maps trip route that persists a
track against the loadout.

> [!NOTE]
> Full reasoning, including why secrets are redacted for widgets but handed to embeds, is
> in [PLUGIN_MODEL.md](PLUGIN_MODEL.md).

---

## Tech stack

- **Frontend**: React 18 (Vite) + Tailwind CSS + lucide-react
- **Backend**: Go, chi router
- **Database**: PostgreSQL (JSONB) with a fully featured in-memory store for local dev
- **Validation**: JSON Schema via a schema registry (write-time validation of namespaces)

## Tests

```bash
cd loadouts/backend && go test ./...            # unit tests
cd loadouts/backend && ./scripts/smoke_day0.sh  # end-to-end against a running server
cd loadouts/backend && ./scripts/smoke_import.sh # end-to-end import flow
cd loadouts/backend && ./scripts/smoke_plugins.sh # end-to-end plugin model
cd loadouts/backend && ./scripts/smoke_favorites.sh # endorsement, staleness, withdrawal
cd loadouts/frontend && npm run build            # type-check + bundle
```

See [ITEM_IMPORT.md](ITEM_IMPORT.md) for the import design notes, [DAY0_MVP_PLAN.md](DAY0_MVP_PLAN.md) for the implementation plan, deferred work, and
acceptance criteria, and [loadouts/backend/TESTING.md](loadouts/backend/TESTING.md) for
manual curl flows.
