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
