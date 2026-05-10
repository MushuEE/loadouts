-- Initial schema for Polyglot Inventory System

CREATE TABLE schemas (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    name TEXT NOT NULL,
    author TEXT NOT NULL,
    definition JSONB NOT NULL,
    is_immutable BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(id, version)
);

CREATE TABLE items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    base_metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_metadata (
    user_id TEXT NOT NULL,
    item_id TEXT NOT NULL REFERENCES items(id),
    overrides JSONB DEFAULT '{}',
    open_data JSONB DEFAULT '{}',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id)
);

-- Community Aggregates Table (Materialized View style for performance)
CREATE TABLE community_aggregates (
    item_id TEXT NOT NULL REFERENCES items(id),
    key_name TEXT NOT NULL,
    agg_value NUMERIC,
    agg_count INTEGER DEFAULT 0,
    PRIMARY KEY (item_id, key_name)
);

-- Indexing for fast global lookups
CREATE INDEX idx_items_base_metadata ON items USING GIN (base_metadata);
CREATE INDEX idx_user_metadata_overrides ON user_metadata USING GIN (overrides);
