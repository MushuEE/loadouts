-- Support for Suppliers and Affiliate Item Sources

CREATE TABLE suppliers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    affiliate_template TEXT NOT NULL -- e.g. "{{.BaseURL}}/dp/{{.ProductID}}?tag=loadouts-20"
);

CREATE TABLE item_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id TEXT NOT NULL REFERENCES items(id),
    supplier_id TEXT NOT NULL REFERENCES suppliers(id),
    product_id TEXT NOT NULL, -- The specific ID for this supplier (e.g. ASIN)
    price NUMERIC(12, 2),
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(supplier_id, product_id) -- Prevent duplicate imports by URL/Product ID
);

CREATE INDEX idx_item_sources_item_id ON item_sources(item_id);

-- Initial Seed for common suppliers
INSERT INTO suppliers (id, name, base_url, affiliate_template) VALUES
('amazon', 'Amazon', 'https://www.amazon.com', '{{.BaseURL}}/dp/{{.ProductID}}?tag=loadouts-20'),
('rei', 'REI', 'https://www.rei.com', '{{.BaseURL}}/product/{{.ProductID}}?cm_mmc=aff_AL-_-loadouts'),
('backcountry', 'Backcountry', 'https://www.backcountry.com', '{{.BaseURL}}/{{.ProductID}}?aff=loadouts');
