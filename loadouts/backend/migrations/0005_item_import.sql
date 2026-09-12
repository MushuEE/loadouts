-- Retailer item importing.
--
-- Two changes:
--   1. Items gain provenance so a scraped catalog entry is distinguishable from a
--      hand-curated one without hiding it. Imports land verified = FALSE.
--   2. item_sources gains the canonical URL and currency, so an item can always be
--      traced back to the page it came from and prices are unambiguous.

ALTER TABLE items
    ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'curated',
    ADD COLUMN IF NOT EXISTS verified BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS imported_by TEXT NOT NULL DEFAULT '';

-- Everything that predates importing was entered by hand, so it stays curated/verified.
-- New imports override these defaults explicitly at insert time.
ALTER TABLE items ALTER COLUMN verified SET DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_items_origin ON items(origin) WHERE origin <> 'curated';
CREATE INDEX IF NOT EXISTS idx_items_unverified ON items(verified) WHERE verified = FALSE;

ALTER TABLE item_sources
    ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'USD';

-- item_sources.id was created as a UUID with a gen_random_uuid() default, but the rest of
-- the Day 0 model uses prefixed text IDs (src_9f2c...). Widen the column so the
-- application can supply its own IDs consistently across both stores.
ALTER TABLE item_sources ALTER COLUMN id TYPE TEXT USING id::TEXT;
ALTER TABLE item_sources ALTER COLUMN id SET DEFAULT gen_random_uuid()::TEXT;

-- Suppliers the importer knows how to read URLs for. The host-matching and product-ID
-- extraction rules live in internal/importer/url.go; this table only holds the affiliate
-- templating, so adding a retailer means a code change plus this row.
INSERT INTO suppliers (id, name, base_url, affiliate_template) VALUES
    ('patagonia', 'Patagonia', 'https://www.patagonia.com', '{{.BaseURL}}/product/{{.ProductID}}.html'),
    ('garagegrowngear', 'Garage Grown Gear', 'https://www.garagegrowngear.com', '{{.BaseURL}}/products/{{.ProductID}}'),
    -- 'other' is the passthrough for retailers we have no rules for. Its product_id is
    -- the full canonical URL, so the template resolves to a plain working link.
    ('other', 'Other', '', '{{.ProductID}}')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    base_url = EXCLUDED.base_url,
    affiliate_template = EXCLUDED.affiliate_template;
