-- Slots an owner added to one loadout, on top of its pinned template version.
--
-- They live here rather than on the template because editing the template would change it
-- for everyone using it, and forking a template for every personal tweak would bury the
-- useful templates under near-identical copies.
--
-- Defaults to an empty array rather than NULL so that core.SlotList always scans into a
-- usable slice and callers never have to distinguish "no custom slots" from "unknown".
ALTER TABLE loadouts
    ADD COLUMN IF NOT EXISTS extra_slots JSONB NOT NULL DEFAULT '[]'::jsonb;
