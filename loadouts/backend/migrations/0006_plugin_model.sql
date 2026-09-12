-- 0006: the plugin model.
--
-- Plugins extend the UI with charts, calculators, tables, and maps. They are authored by
-- untrusted users and communities, so the schema is built around two ideas: versions are
-- immutable, and everything a plugin can reach is namespaced to it.

CREATE TABLE IF NOT EXISTS plugins (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    -- Mirrors templates: a plugin belongs to the platform, a profile, or a community.
    owner_type      TEXT NOT NULL CHECK (owner_type IN ('platform', 'profile', 'community')),
    owner_id        TEXT NOT NULL DEFAULT '',
    latest_version  INTEGER NOT NULL DEFAULT 0,
    is_public       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugins_owner ON plugins(owner_type, owner_id);
CREATE INDEX IF NOT EXISTS idx_plugins_public ON plugins(is_public) WHERE is_public;

-- A published manifest. There is no UPDATE path for this table by design: an install
-- pinned to v1 must keep behaving exactly as it did when the installer approved it, so
-- publishing a change means inserting v2.
CREATE TABLE IF NOT EXISTS plugin_versions (
    plugin_id   TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    manifest    JSONB NOT NULL,
    changelog   TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plugin_id, version)
);

-- A GIN index over the manifest makes the "which plugins render into this surface?"
-- lookup cheap, since surface lives inside the views array rather than in a column.
CREATE INDEX IF NOT EXISTS idx_plugin_versions_manifest ON plugin_versions USING GIN (manifest);

-- A plugin enabled in a scope. Installs are opt-in per profile; a community admin
-- installs on behalf of a community.
CREATE TABLE IF NOT EXISTS plugin_installs (
    id            TEXT PRIMARY KEY,
    plugin_id     TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    -- The pinned version. Upgrading is an explicit act, not something a publish does.
    version       INTEGER NOT NULL,
    scope_type    TEXT NOT NULL CHECK (scope_type IN ('profile', 'community')),
    scope_id      TEXT NOT NULL,
    -- What was approved. A manifest asking for more after an upgrade forces a re-grant
    -- instead of silently widening access.
    granted_caps  JSONB NOT NULL DEFAULT '[]'::JSONB,
    settings      JSONB NOT NULL DEFAULT '{}'::JSONB,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    installed_by  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (plugin_id, version) REFERENCES plugin_versions(plugin_id, version)
);

-- One install per (scope, plugin): re-installing upgrades in place rather than stacking
-- duplicate copies of the same plugin on one profile.
CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_installs_scope_plugin
    ON plugin_installs(scope_type, scope_id, plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_installs_scope ON plugin_installs(scope_type, scope_id);

-- Plugin-owned storage: a route plugin saving a GPX track against a loadout, a calculator
-- remembering its inputs. The composite key is the isolation boundary - it is what keeps
-- one plugin from reading another's data, and one loadout's data from leaking into
-- another's.
CREATE TABLE IF NOT EXISTS plugin_data (
    plugin_id   TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    scope_type  TEXT NOT NULL CHECK (scope_type IN ('loadout', 'item', 'community', 'profile')),
    scope_id    TEXT NOT NULL,
    key         TEXT NOT NULL,
    value       JSONB NOT NULL DEFAULT '{}'::JSONB,
    updated_by  TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plugin_id, scope_type, scope_id, key)
);

CREATE INDEX IF NOT EXISTS idx_plugin_data_scope ON plugin_data(scope_type, scope_id);
