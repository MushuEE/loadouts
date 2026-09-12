# Local Testing Guide

This guide explains how to verify the Loadouts backend locally.

## Prerequisites
- **Go 1.25+**
- **curl** and **python3** (for the smoke script)
- **git**

## 1. Unit Testing

Layer resolution, template versioning, loadout validation/stats/forking, and community
role gating are covered by Go unit tests.

```bash
cd loadouts/backend
go test ./...
```

## 2. End-to-End Smoke Test

`scripts/smoke_day0.sh` walks the whole Day 0 object model against a running server:
layer resolution, private-layer redaction, nested loadout entries, template versioning,
forking, visibility, and community admin gating.

```bash
# Terminal 1: the server auto-seeds the demo dataset when using the in-memory store
cd loadouts/backend
go run ./cmd/server

# Terminal 2
cd loadouts/backend
./scripts/smoke_day0.sh
```

`scripts/seed.sh` remains available and exercises the original schema-registry and
item-override endpoints.

## 3. Manual API Exploration

Every request acts as a profile via the `X-Profile-ID` header (an ID or a `@handle`).

```bash
API=http://localhost:8080/api/v1

# Who exists?
curl -s $API/profiles

# Global view of an item: only the base layer applies.
curl -s $API/items/tent-copper-spur-ul2

# Through the UL community's lens, as the owning profile: all four layers apply, and the
# response's provenance map says which layer produced each value.
curl -s -H "X-Profile-ID: gearhead" \
  "$API/items/tent-copper-spur-ul2?community=ul-backpacking"

# Same item, viewed by a different profile: the private layer is gone.
curl -s -H "X-Profile-ID: trailsponsor" \
  "$API/items/tent-copper-spur-ul2?community=ul-backpacking&owner=<gearhead-profile-id>"

# The public feed, and one loadout in full (nested entries + stats + validation).
curl -s $API/discover
curl -s -H "X-Profile-ID: gearhead" $API/loadouts/<loadout-id>

# Fork someone else's loadout into your own account.
curl -s -X POST -H "X-Profile-ID: trailsponsor" $API/loadouts/<loadout-id>/fork
```

## 4. Key Behaviors to Verify

- **Layer precedence**: `global → community → user public → user private`. A user's public
  override beats the community layer, which beats the global item.
- **Private redaction**: the `user_private` layer must never appear in a response where the
  viewer is not the owning profile. `applied_layers` tells you which layers were used.
- **Community scoping**: community metadata must not appear without `?community=`, and must
  not appear for a *different* community.
- **Template immutability**: publishing a new version bumps `latest_version` but leaves older
  versions byte-identical. A loadout pinned to v1 keeps validating against v1.
- **Publish gating**: a loadout with an unfilled *required* slot may be saved as a draft
  (warning) but must fail to publish (error).
- **Visibility**: a private loadout returns `403` for anyone but its owner, including
  anonymous callers.
- **Admin gating**: only community admins/owners can write a community item layer (`403`
  otherwise), and the last owner cannot leave a community.
- **Nesting**: entries with a `parent_entry_id` roll up into the parent container's subtree,
  and their weight/cost still counts toward the loadout totals.
- **Schema validation**: a public user layer whose namespace matches a registered schema must
  conform to it; unregistered namespaces are allowed through.

---

## 5. Item Importing

```bash
./scripts/smoke_import.sh    # 10-section end-to-end run
```

The script does not depend on any real store being reachable — that is the point of the
design. Retailers *will* block us, and every interesting behavior still has to work.

### Manual flows

```bash
API=http://localhost:8080/api/v1
GH=$(curl -s "$API/profiles" | python3 -c "import sys,json;print([p['id'] for p in json.load(sys.stdin) if p['handle']=='gearhead'][0])")

# Preview: writes nothing, safe to call repeatedly
curl -s -X POST "$API/imports/preview" -H 'Content-Type: application/json' \
  -d '{"url":"https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2"}' | python3 -m json.tool

# Commit the confirmed draft
curl -s -X POST "$API/imports/commit" -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"url":"https://www.rei.com/product/894303/x","name":"Copper Spur UL2","category":"shelter","weight_g":1360,"cost_cents":54995}' \
  | python3 -m json.tool
```

### Key behaviors to verify

- **Canonicalization is network-free**: `rei.com/product/894303/any-slug?utm_source=x`,
  `www.rei.com/product/894303/other-slug`, and the bare `rei.com/product/894303` must all
  resolve to `(rei, 894303)` and therefore to the *same* catalog item.
- **Graceful degradation**: a retailer returning 403/503 must produce `status: "manual"`
  with a correct `product_id` and `affiliate_url` — never a 5xx from our API.
- **SSRF is a hard failure**: `http://169.254.169.254/…`, `http://localhost:…`,
  `http://10.0.0.1/…`, and `http://x.internal/…` must all return `403`, on **both**
  `/imports/preview` and `/imports/commit` (commit never fetches, so it is guarded
  separately in `Canonicalize`).
- **Idempotency**: committing the same product twice returns `201` then `200`, with
  `created: false` and the same item ID the second time. The catalog must not grow.
- **Unknown weight ≠ zero weight**: when the page has no weight, `has_weight` must be
  `false` and the UI field must be blank, not `0`.
- **Provenance**: imported items carry `origin: "import"`, `verified: false`,
  `imported_by: <profileID>`, and an `import` metadata namespace with the supplier,
  product ID, and source URL.
- **Auth**: `/imports/commit` requires `X-Profile-ID` and returns `403` without it.
  `/imports/preview` is anonymous-safe.
- **Non-product URLs**: a known retailer's search or category page returns `400` with a
  message naming the retailer, not a generic parse failure.
- **Stats integration**: an imported item added to a loadout must move
  `total_weight_g` / `total_cost_cents`. This is the real test of whether the import was
  worth anything.

---

## 6. The Plugin Model

```bash
./scripts/smoke_plugins.sh   # 11-section end-to-end run
```

The script is organized around a single question: what can an untrusted plugin author
actually do? Each section is either something they are allowed, something they are
refused, or something they are handed only once the frame is isolated from everything
else.

### Manual flows

```bash
API=http://localhost:8080/api/v1
GH=$(curl -s "$API/profiles" | python3 -c "import sys,json;print([p['id'] for p in json.load(sys.stdin) if p['handle']=='gearhead'][0])")
LDT=$(curl -s "$API/loadouts" | python3 -c "import sys,json;print([l['loadout']['id'] for l in json.load(sys.stdin) if l['owner_handle']=='gearhead'][0])")

# The directory, and one surface rendered over real data
curl -s "$API/plugins" | python3 -m json.tool
curl -s -H "X-Profile-ID: $GH" "$API/plugins/render?surface=loadout.panel&loadout_id=$LDT" | python3 -m json.tool

# Publish a widget of your own
curl -s -X POST "$API/plugins" -H "X-Profile-ID: $GH" -H 'Content-Type: application/json' -d '{
  "name": "Cost per gram",
  "manifest": {
    "api_version": 1,
    "capabilities": {"read_loadout": true, "read_items": true},
    "views": [{
      "id": "cpg", "title": "Cost per gram", "surface": "loadout.panel", "kind": "widget",
      "widget": {"type": "stat_grid", "source": "loadout.entries", "stats": [
        {"label": "Total", "value": "sum(item.cost_cents * entry.quantity)", "format": "currency_cents"},
        {"label": "Per gram", "value": "sum(item.cost_cents * entry.quantity) / sum(item.weight_g * entry.quantity)", "format": "number"}
      ]}
    }]
  }, "is_public": true}' | python3 -m json.tool
```

### Key behaviors to verify

- **A bad manifest is a publish-time `400`.** Unparseable expressions, unknown functions,
  nested aggregates, unknown surfaces, and embeds with no HTML must all be refused by
  `POST /plugins`. Anything that reaches the renderer is a bug: the promise is that
  readers never see a broken plugin an author could have been told about.
- **A partial capability grant is refused.** Installing a plugin while granting a subset
  of what its manifest asks for returns `400`, never a silent downgrade.
- **Installs pin a version.** After the author publishes v2, an existing install must
  still render v1, report `upgrade_available: true`, and list any new capability in
  `missing_caps`.
- **Secrets are redacted for widgets and real for embeds.** A `secret` setting reads
  `••••••` in `GET /plugins/installs` but appears verbatim in an embed view's
  `embed_context.settings`. Both are correct; see PLUGIN_MODEL.md.
- **Install listings are scoped and authorized.** An unscoped request is `400`, and one
  profile reading another's installs is `403`. (Regression: the first version of this
  endpoint returned every install on the platform to anonymous callers.)
- **The frame is sandboxed without `allow-same-origin`.** Check the response headers of
  `/sandbox/.../frame` for a `default-src 'none'` CSP whose `script-src`/`connect-src`
  contain only the hosts the install granted, plus `frame-ancestors` naming `APP_ORIGIN`.
- **The frame document contains no data.** The render context arrives by `postMessage`
  from the parent; the HTML itself should hold no loadout IDs and no API key.
- **Ungranted reads are absent, not empty.** A plugin without `items:read` must have no
  `items` key in its context at all.
- **Rendering respects loadout visibility.** A plugin surface on a private loadout is
  `200` for the owner and `403` for anyone else — installing a plugin must not become a
  way to read what you otherwise could not.
- **One broken plugin is one broken card.** A view that fails returns a `RenderedView`
  carrying `error`; the surface request itself still returns `200` with the other views
  intact.
