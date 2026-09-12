-- Recursive templates: a loadout entry may reference another loadout instead of an item.
--
-- "Meals Day 1" hanging off a trip's food slot is a reference to an independent, 
-- publishable, forkable loadout, not a part of the trip. That makes loadouts a directed
-- graph; the acyclicity and depth invariants are enforced in the service on write, since
-- they are not expressible as column constraints.

-- An entry is now one kind or the other, so an item is no longer always present.
-- The foreign key stays: item references must still be real.
ALTER TABLE loadout_entries ALTER COLUMN item_id DROP NOT NULL;

-- child_loadout_id deliberately has NO foreign key.
--
-- We want the reference to outlive its target. Deleting "Meals Day 2" should leave the
-- trip's food entry in place so the UI can say "this sub-loadout is gone" and the
-- validator can raise an issue, rather than the slot silently emptying itself. ON DELETE
-- CASCADE would erase the evidence, and ON DELETE SET NULL would produce a row that
-- satisfies neither half of the check constraint below, making the delete fail outright.
-- Integrity here is the application's job precisely because dangling is a state we want
-- to represent.
ALTER TABLE loadout_entries ADD COLUMN child_loadout_id TEXT NOT NULL DEFAULT '';

-- Selected marks the active child of an "alternatives" slot. Everything is selected by
-- default, which is what a "sum" slot means and what every pre-existing row wants.
ALTER TABLE loadout_entries ADD COLUMN selected BOOLEAN NOT NULL DEFAULT true;

-- The reverse index. The depth check asks "what references this loadout?" on every
-- attachment, because nesting a subtree breaks the invariant for everything above the
-- parent, not just the parent.
CREATE INDEX idx_loadout_entries_child
    ON loadout_entries(child_loadout_id)
    WHERE child_loadout_id <> '';

-- Exactly one kind, never both and never neither.
ALTER TABLE loadout_entries ADD CONSTRAINT entry_item_xor_child_loadout CHECK (
    (item_id IS NOT NULL AND child_loadout_id = '')
    OR
    (item_id IS NULL AND child_loadout_id <> '')
);
