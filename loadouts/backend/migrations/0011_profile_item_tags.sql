-- Tags are per-profile (#34).
--
-- A tag is one profile's label for one item: "#warmwear" is @gearhead's opinion of their
-- Capilene, not a fact about the Capilene. So tags live in their own table rather than in
-- a metadata layer, where the global and community layers could merge in tags nobody chose.
-- The global view of a tag is an aggregate over this table, never a stored value.
CREATE TABLE IF NOT EXISTS profile_item_tags (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag        TEXT NOT NULL,
    position   INT  NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (profile_id, item_id, tag)
);

-- The aggregates: how many people tag this item with what, and which tags are popular.
CREATE INDEX IF NOT EXISTS idx_profile_item_tags_item ON profile_item_tags (item_id);
CREATE INDEX IF NOT EXISTS idx_profile_item_tags_tag ON profile_item_tags (tag);
