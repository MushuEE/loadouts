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

GET    /api/v1/items                       POST /api/v1/items
GET    /api/v1/items/{id}?community=&owner=
GET/PUT /api/v1/items/{id}/layers/profile
GET    /api/v1/schemas                     POST /api/v1/schemas
```

---

## Tech stack

- **Frontend**: React 18 (Vite) + Tailwind CSS + lucide-react
- **Backend**: Go, chi router
- **Database**: PostgreSQL (JSONB) with a fully featured in-memory store for local dev
- **Validation**: JSON Schema via a schema registry (write-time validation of namespaces)

## Tests

```bash
cd loadouts/backend && go test ./...          # unit tests
cd loadouts/backend && ./scripts/smoke_day0.sh # end-to-end against a running server
cd loadouts/frontend && npm run build          # type-check + bundle
```

See [DAY0_MVP_PLAN.md](DAY0_MVP_PLAN.md) for the implementation plan, deferred work, and
acceptance criteria, and [loadouts/backend/TESTING.md](loadouts/backend/TESTING.md) for
manual curl flows.
