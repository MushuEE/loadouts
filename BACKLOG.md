# Backlog

Work that is deliberately deferred, with enough context to pick it up cold.

Ordered roughly by the value-to-effort ratio as understood today. Nothing here is
committed to a milestone.

---

## Recursive templates — Phase 3 (in flight, not deferred)

Tracked in [#13](https://github.com/MushuEE/loadouts/issues/13): service-layer
attach/detach/select, the recursive `Detail` tree walk, and the stats rollup, with the
`Fork` question that still needs a decision. Phases 1–2 (core types, reference graph,
storage) are merged; [RECURSIVE_TEMPLATES_PLAN.md](RECURSIVE_TEMPLATES_PLAN.md) has the
full eight-phase breakdown.

Listed here for discoverability only — unlike everything below it, this is next up rather
than parked.

---

## Purchase from a loadout (affiliate codes)

**Status:** designed far enough to know the shape, not started.

The goal: a "Buy" affordance on every item in a loadout, with a monetized, attributable
outbound link.

### What already exists

- `suppliers.affiliate_template` and `core.ResolveSourceURL(supplier, source)` render a
  product URL from a template.
- `item_sources` rows are written by the importer, and `ResolveItem` attaches
  `[]ResolvedSource{SupplierName, Price, URL}` to every resolved item.
- Loadout detail already carries those sources: `LoadoutService` resolves entries through
  `InventoryService.ResolveItems`, so the data needed for a buy button is **already on the
  wire** for every loadout entry.

### Findings from the initial investigation

> [!NOTE]
> These come from reading the code, not from a reproduction. Verify before relying on them.

1. **Affiliate codes are hardcoded into template strings in four places** —
   `internal/importer/url.go` (the registry), `internal/db/memory.go` (the memory store's
   seeded suppliers), `internal/service/import.go` (the generic-supplier fallback), and
   `migrations/0003_add_item_sources.sql`. Rotating `loadouts-20` today means editing all
   four and shipping a migration. The code needs to become a stored field, separate from
   the template, ideally overridable by deployment config.

2. **Nothing renders `sources` in the frontend yet.** A grep across `loadouts/frontend/src`
   returns no uses outside the type definition, so the buy button is genuinely net-new UI
   rather than a rewire.

3. **`ResolveSourceURL` does no URL escaping.** It is `text/template` interpolating
   `{{.ProductID}}` straight into a URL string. Benign for the alphanumeric IDs we see in
   practice (ASINs, REI numbers), but it means a product ID containing `&` or `?` would
   corrupt the query string.

4. **The generic supplier's product ID is a full URL, and that URL may already have a
   query string.** `Canonicalize` strips only *known* tracking params, so
   `shop.example.com/p/widget?variant=42` survives intact and becomes the `other`
   supplier's `ProductID`. Naively appending `?tag=…` to a rendered URL would therefore
   produce a second `?`. Affiliate injection has to parse the URL and merge into
   `url.Values` rather than concatenate strings.

### Open product questions (need a decision before building)

- **Whose code gets injected?** The object model has Creators and Sponsors, so this is a
  revenue-sharing decision, not just a technical one:
  - platform code always (simplest, all revenue to the platform), or
  - a resolution chain — the loadout owner's own code if they've registered one for that
    supplier, else the community's, else the platform's, or
  - sponsor-specific codes attached to particular items.
- **Attribution.** "Purchase *from a loadout*" implies wanting to know which loadout
  converted. Most affiliate programs support a sub-ID parameter (Amazon `ascsubtag`,
  others `subid`) — encoding loadout + profile there is what makes the feature
  measurable. Note the charset and length limits these programs impose.
- **Redirect-style networks.** REI is on an affiliate network rather than a plain query
  param; those use a wrapper URL (`…/deeplink?id=CODE&murl=<encoded product url>`). If we
  want real REI revenue the renderer needs to support wrapping, not just param injection.
- **Disclosure.** Affiliate links generally carry a legal disclosure requirement. Worth
  settling where that surfaces in the UI before shipping.

### Sketch

Store the code as data rather than baking it into the template:

```go
type Supplier struct {
    // …
    AffiliateTemplate string // code-free: "{{.BaseURL}}/dp/{{.ProductID}}"
    AffiliateCode     string // "loadouts-20"
    AffiliateParam    string // "tag"
    SubIDParam        string // "ascsubtag"
    Enabled           bool
}
```

…then render the template, `url.Parse` the result, merge the code and sub-ID into
`url.Values`, and re-encode. That fixes findings 3 and 4 as a side effect.

---

## Item importing follow-ups

See [ITEM_IMPORT.md](ITEM_IMPORT.md) for the full rationale.

| Item | Notes |
| --- | --- |
| Browser extension / bookmarklet | The only real fix for Amazon, which blocks server-side fetches. The user's browser has their cookies and isn't treated as a bot. Larger surface area than the server-side path. |
| Scheduled price refresh | `item_sources.last_updated` and the upsert path already exist for it; there is no scheduler. |
| Bulk import | Paste many URLs, review as a list. The service API is already shaped for it. |
| Moderation queue for unverified imports | The `verified` flag and its partial index exist; no review UI. |
| Amazon Product Advertising API | Needs an approved affiliate account (3 sales in 180 days). The URL path works today without it. |
| Image rehosting | We hotlink the retailer's image URL. |

---

## Platform

| Item | Notes |
| --- | --- |
| Real authentication | Day 0 auth is a single `X-Profile-ID` header with no verification. Everything downstream already threads an acting profile, so this is a middleware swap plus a login UI. |
| Redis merge cache | Layer resolution is recomputed per request. Fine at current scale; the merge is pure and keyed by `(item, community, owner, viewer)`, so it caches cleanly when needed. |
| Template migration functions | Moving a loadout from template v1 to v2 currently has no assisted path — slots that disappeared or changed categories need manual fixing. |

---

## Known rough edges

- `item_sources.price` is a float; money elsewhere in the model is integer cents
  (`core.cost_cents`). Worth unifying.
- The legacy `POST /items/{id}/metadata` endpoint and `core.UserMetadata` shape are kept
  as a compat shim over profile layers. They can go once nothing depends on them.
- `MemoryStore` and the Postgres migrations seed slightly different supplier sets;
  `ImportService.Commit` papers over this by upserting from the code registry on first use.
