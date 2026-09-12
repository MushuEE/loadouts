# Inventory System Implementation Plan

## Goal
Build a flexible inventory metadata system that supports:
- **Global Metadata Store**: A master database of items and their default metadata.
- **User Metadata Store**: User-specific custom items or overrides/extensions of global items.
- **Polyglot/Flexible Metadata**: Support for an arbitrary array of metadata fields per item.

## Current Architecture Ideas
- **Language**: Go (Backend)
- **Database**: PostgreSQL with `JSONB` for flexible metadata storage.
- **Caching**: Redis for "Merged Result" caching (`merged_{userId}_{itemId}`).
- **Scaling**: 500k DAU, 20k-50k QPS peak targets.
- **Metadata Separation**:
    - **Aggregate Layer (Shared)**: Social metrics (ratings, comfort) pre-computed for global sorting.
    - **Private Layer (User-Specific)**: Personal notes, tags, partitioned by `user_id`.
- **Plugin System**: 
    - **Namespacing**: Metadata blocks keyed by plugin/community (e.g., `osrs_plugin`).
    - **Schema Registry**: Plugins register schemas to define searchable/sortable fields.

## Grilling Progress
1. **Data Modeling**: Decided on PostgreSQL + JSONB for a balance of flexibility and point-lookup speed.
2. **Scaling**: Decided on a caching layer for fully merged objects to handle high QPS.
3. **Shared vs Private**: Decided to pre-compute aggregate metrics for sorting while keeping personal data private.
4. **Plugins**: Decided on namespacing and a registry for plugin metadata.

## Finalized Architecture
- **Validation**: Strict Write-time validation via a **Schema Registry**. Data must conform to a schema before being written to `plugins` namespaces.
- **OpenSchema**: A dedicated "Generic" namespace for unstructured, private, and unindexed key-value pairs (`key:<any>`).
- **Layered Namespaces**: 
    - **Global Store**: Base item data.
    - **User Store**: User-specific overrides/deltas.
    - **Deep Merge**: Backend merges layers on-the-fly (cached in Redis).
- **Bloom Filter**: 
    - **Current**: In-memory "Negative Cache" to prevent ID collisions and unnecessary DB lookups.
    - **Production Scaling**: Transition to **Redis Bloom** for a shared, persistent Bloom filter across multiple backend instances.
- **Search Engine**: 
    - **Global Search**: Restricted to Global/Pre-aggregated data.
    - **Community Stats**: Materialized aggregates (e.g., average ratings) updated via eventually consistent event workers.
- **Schema Evolution**: Immutable versions + "Lazy Migration on Read" using author-provided migration functions.
- **Images**:
    - **Global**: `image_url` as a first-class column in the `items` table.
    - **User Override**: `custom_image_url` in `user_metadata` for personal item photos.
    - **Optimization**: Use "Resize-on-the-fly" via URL query parameters (e.g., `?w=64`) for grid thumbnails.
- **Item Sources & Affiliates**:
    - **Suppliers**: Central registry for major retailers (Amazon, REI, etc.) with affiliate link templates.
    - **Indexing**: `item_sources` table acts as a reverse-lookup (URL/Product ID -> Item) to prevent duplicate imports.
    - **Dynamic Pricing**: Cached prices with a TTL; updated via background refresh logic.
    - **Monetization**: Final purchase URLs are generated on-the-fly by injecting item-specific IDs into supplier-level affiliate templates.

## Day 0 MVP Decisions (see DAY0_MVP_PLAN.md)

The inventory work above was only the `Item` half of the product. Day 0 added the rest of the
object model — `User`, `Profile`, `Community`, `Template`, `Loadout` — and generalized the
two-layer merge into the full four-layer model.

- **Four metadata layers**: `global -> community -> user public -> user private`. The private
  layer is redacted inside the service (not the handler) so a new endpoint cannot leak it.
  Every resolution returns a `provenance` map (`namespace.key -> layer`) so the UI can show
  *why* a value looks the way it does.
    - The old `user_metadata` table folded into `profile_item_layers`
      (`overrides -> public_metadata`, `open_data -> private_metadata`); the legacy
      `/items/{id}/metadata` endpoint stays as a compat shim over the new store.
- **Community scoping**: a community layer only applies when the item is read with that
  community's context. This keeps hobby-specific attributes (`ul_score`) out of the global
  record while still making them first-class inside the community.
- **`core` namespace**: one platform-owned namespace (`weight_g`, `cost_cents`, `consumable`)
  gives the stats engine something universal to sum, without constraining any hobby's own
  namespaces.
- **Immutable template versions**: publishing appends a version and moves a pointer; loadouts
  pin `(template_id, template_version)`. An author can evolve a template freely and nobody
  else's loadout breaks — which also means we don't need migration functions yet.
- **Whole-tree entry writes**: `PUT /loadouts/{id}/entries` replaces the entire entry set.
  The editor already owns the full client-side state, so this keeps writes atomic and the
  client trivial, at the cost of larger payloads (fine at Day 0 sizes).
- **Nesting via `parent_entry_id`**: preserves the telescoping pack → pocket → ditty bag UX
  while keeping the storage flat and easy to query.
- **Draft vs publish validation**: template violations are warnings while drafting and
  blocking errors at publish time, so the editor never fights the user mid-build.
- **Dev auth**: `X-Profile-ID` resolved in `internal/auth`, a deliberate single swap point for
  real authentication. The UI's profile switcher is the login screen.
- **Frontend is now API-backed**: the mock database was removed; when the API is unreachable
  the app says so explicitly rather than silently showing fake data.

