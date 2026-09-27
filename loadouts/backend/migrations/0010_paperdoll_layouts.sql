-- Grid paperdolls (#30).
--
-- The layout is part of the immutable template version, like its slots, so a loadout
-- pinned to v1 keeps the paperdoll it was built against when v2 repaints it. NULL means
-- the version has no layout and the client infers placement from slot categories (#31).
ALTER TABLE template_versions
    ADD COLUMN IF NOT EXISTS paperdoll JSONB;

-- Site admins author figure paperdolls. There is no API to grant this; it is set by the
-- seed or directly in the database until there is an admin console.
ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS is_site_admin BOOLEAN NOT NULL DEFAULT FALSE;
