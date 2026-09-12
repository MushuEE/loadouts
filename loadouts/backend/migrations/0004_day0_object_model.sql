-- Day 0 MVP: the full Loadouts object model.
--
-- Adds identity (users/profiles), communities + membership, the community and profile
-- metadata layers, versioned templates, and loadouts with (optionally nested) entries.
-- Also folds the old `user_metadata` table into `profile_item_layers`, where
--   overrides  -> public_metadata   (publicly visible user layer)
--   open_data  -> private_metadata  (owner-only user layer)

-- ---------------------------------------------------------------- Items (extensions)

ALTER TABLE items ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'universal';
-- Container items (packs, ditty bags, pots) expose their own slots for the telescoping UI.
ALTER TABLE items ADD COLUMN IF NOT EXISTS provided_slots JSONB NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_items_category ON items(category);

-- ---------------------------------------------------------------- Identity

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- A User owns 1..n Profiles. All authored content hangs off a Profile, never a User.
CREATE TABLE profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    handle TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    bio TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    is_sponsor BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_profiles_user_id ON profiles(user_id);

-- ---------------------------------------------------------------- Communities

CREATE TABLE communities (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by TEXT REFERENCES profiles(id),
    member_count INTEGER NOT NULL DEFAULT 0,
    metadata_hint JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE community_members (
    community_id TEXT NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member', -- member | admin | owner
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (community_id, profile_id)
);

CREATE INDEX idx_community_members_profile ON community_members(profile_id);

-- ---------------------------------------------------------------- Metadata layers

-- Community layer: public, but only applied when an item is viewed in this community's scope.
CREATE TABLE community_item_layers (
    community_id TEXT NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    metadata JSONB NOT NULL DEFAULT '{}',
    updated_by TEXT REFERENCES profiles(id),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (community_id, item_id)
);

-- Profile layer: public_metadata is world-readable, private_metadata is owner-only.
CREATE TABLE profile_item_layers (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    custom_image_url TEXT NOT NULL DEFAULT '',
    public_metadata JSONB NOT NULL DEFAULT '{}',
    private_metadata JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (profile_id, item_id)
);

CREATE INDEX idx_community_item_layers_item ON community_item_layers(item_id);
CREATE INDEX idx_profile_item_layers_item ON profile_item_layers(item_id);
CREATE INDEX idx_profile_item_layers_public ON profile_item_layers USING GIN (public_metadata);

-- ---------------------------------------------------------------- Templates

CREATE TABLE templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_type TEXT NOT NULL DEFAULT 'profile', -- platform | profile | community
    owner_id TEXT NOT NULL DEFAULT '',
    community_id TEXT NOT NULL DEFAULT '',
    latest_version INTEGER NOT NULL DEFAULT 1,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Versions are immutable: publishing a change adds a row, it never updates one.
-- This is what keeps existing loadouts from breaking when a template evolves.
CREATE TABLE template_versions (
    template_id TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    slots JSONB NOT NULL DEFAULT '[]',
    changelog TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (template_id, version)
);

CREATE INDEX idx_templates_community ON templates(community_id);

-- ---------------------------------------------------------------- Loadouts

CREATE TABLE loadouts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    community_id TEXT NOT NULL DEFAULT '',
    template_id TEXT NOT NULL REFERENCES templates(id),
    template_version INTEGER NOT NULL DEFAULT 1,
    visibility TEXT NOT NULL DEFAULT 'private', -- private | unlisted | public
    status TEXT NOT NULL DEFAULT 'draft',       -- draft | published
    forked_from TEXT NOT NULL DEFAULT '',
    cover_image_url TEXT NOT NULL DEFAULT '',
    fork_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- parent_entry_id gives us the telescoping tree: pack -> side pocket -> ditty bag -> item.
CREATE TABLE loadout_entries (
    id TEXT PRIMARY KEY,
    loadout_id TEXT NOT NULL REFERENCES loadouts(id) ON DELETE CASCADE,
    slot_id TEXT NOT NULL,
    parent_entry_id TEXT NOT NULL DEFAULT '',
    item_id TEXT NOT NULL REFERENCES items(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    note TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_loadouts_owner ON loadouts(owner_profile_id);
CREATE INDEX idx_loadouts_community ON loadouts(community_id);
CREATE INDEX idx_loadouts_visibility ON loadouts(visibility, updated_at DESC);
CREATE INDEX idx_loadout_entries_loadout ON loadout_entries(loadout_id);
CREATE INDEX idx_loadout_entries_parent ON loadout_entries(parent_entry_id);

-- ---------------------------------------------------------------- Migrate legacy user_metadata
-- The old table keyed on an opaque user_id. Day 0 scopes the user layer to a Profile, so we
-- only carry over rows whose user_id already resolves to a profile; the prototype dataset is
-- otherwise regenerated from the seed.

INSERT INTO profile_item_layers (profile_id, item_id, custom_image_url, public_metadata, private_metadata, updated_at)
SELECT um.user_id, um.item_id, COALESCE(um.custom_image_url, ''), um.overrides, um.open_data, um.updated_at
FROM user_metadata um
JOIN profiles p ON p.id = um.user_id
ON CONFLICT (profile_id, item_id) DO NOTHING;

DROP TABLE IF EXISTS user_metadata;
