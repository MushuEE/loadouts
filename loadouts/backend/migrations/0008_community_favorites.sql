-- 0008_community_favorites.sql
--
-- A favorite is a durable pointer at a loadout the holder does not own: a community
-- endorsing "this is the good meal kit", or a profile bookmarking one for later.
--
-- This replaces the community-owned-loadout design (see COMMUNITY_FAVORITES.md §1). The
-- difference that matters: a favorite confers no authority. Nothing consults it to decide
-- whether someone may edit something, so it never grows permission checks.

CREATE TABLE IF NOT EXISTS favorites (
    id               TEXT PRIMARY KEY,
    scope_type       TEXT NOT NULL CHECK (scope_type IN ('profile', 'community')),
    scope_id         TEXT NOT NULL,
    loadout_id       TEXT NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    -- Digest of the loadout's substance when this was last confirmed. Comparing it to the
    -- loadout's current digest is what surfaces "edited since endorsed".
    fingerprint      TEXT NOT NULL DEFAULT '',
    -- Who endorsed, or last re-confirmed. For a community that is the acting admin.
    actor_profile_id TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    confirmed_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Favoriting twice is a re-confirmation, not a second row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_favorites_unique
    ON favorites(scope_type, scope_id, loadout_id);

-- "Who endorsed this loadout?" on the loadout page.
CREATE INDEX IF NOT EXISTS idx_favorites_loadout ON favorites(loadout_id);

-- "This community's shelf", newest endorsement first.
CREATE INDEX IF NOT EXISTS idx_favorites_scope
    ON favorites(scope_type, scope_id, confirmed_at DESC);

-- No foreign key on loadout_id, deliberately, matching the house style for soft
-- references. The row must outlive its target: a community that endorsed six kits and
-- now sees five should be told that one went away, not silently shown five. A cascade
-- would erase exactly the evidence the UI needs.
