package db

import (
	"context"
	"fmt"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type PostgresStore struct {
	db *sqlx.DB
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sqlx.Connect("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) CreateSchema(ctx context.Context, schema core.SchemaDefinition) error {
	query := `INSERT INTO schemas (id, version, name, author, definition, is_immutable)
	          VALUES (:id, :version, :name, :author, :definition, :is_immutable)`
	_, err := s.db.NamedExecContext(ctx, query, schema)
	return err
}

func (s *PostgresStore) GetSchema(ctx context.Context, id, version string) (core.SchemaDefinition, error) {
	var schema core.SchemaDefinition
	query := `SELECT * FROM schemas WHERE id = $1 AND version = $2`
	err := s.db.GetContext(ctx, &schema, query, id, version)
	return schema, err
}

func (s *PostgresStore) ListSchemas(ctx context.Context) ([]core.SchemaDefinition, error) {
	var schemas []core.SchemaDefinition
	query := `SELECT * FROM schemas`
	err := s.db.SelectContext(ctx, &schemas, query)
	return schemas, err
}

func (s *PostgresStore) CreateItem(ctx context.Context, item core.Item) error {
	query := `INSERT INTO items (id, name, category, image_url, provided_slots, base_metadata)
	          VALUES (:id, :name, :category, :image_url, :provided_slots, :base_metadata)`
	_, err := s.db.NamedExecContext(ctx, query, item)
	return err
}

func (s *PostgresStore) GetItem(ctx context.Context, id string) (core.Item, error) {
	var item core.Item
	query := `SELECT * FROM items WHERE id = $1`
	err := s.db.GetContext(ctx, &item, query, id)
	return item, err
}

func (s *PostgresStore) ListItems(ctx context.Context, searchTerm string) ([]core.Item, error) {
	items := []core.Item{}
	// Very basic search for now. In the future, this could use GIN index or ElasticSearch.
	query := `SELECT * FROM items WHERE name ILIKE $1 OR id ILIKE $1 OR category ILIKE $1 ORDER BY name`
	err := s.db.SelectContext(ctx, &items, query, "%"+searchTerm+"%")
	return items, err
}

// UpdateUserMetadata is the legacy compat shim: it writes the profile item layer, mapping
// overrides -> public_metadata and open_data -> private_metadata.
func (s *PostgresStore) UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error {
	return s.UpsertProfileItemLayer(ctx, core.ProfileItemLayer{
		ProfileID:       meta.UserID,
		ItemID:          meta.ItemID,
		CustomImageURL:  meta.CustomImageURL,
		PublicMetadata:  meta.Overrides,
		PrivateMetadata: meta.OpenData,
	})
}

func (s *PostgresStore) GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error) {
	layer, err := s.GetProfileItemLayer(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}
	return &core.UserMetadata{
		UserID:         layer.ProfileID,
		ItemID:         layer.ItemID,
		CustomImageURL: layer.CustomImageURL,
		Overrides:      layer.PublicMetadata,
		OpenData:       layer.PrivateMetadata,
		UpdatedAt:      layer.UpdatedAt,
	}, nil
}

func (s *PostgresStore) GetItemSources(ctx context.Context, itemID string) ([]core.ItemSource, error) {
	var sources []core.ItemSource
	query := `SELECT * FROM item_sources WHERE item_id = $1`
	err := s.db.SelectContext(ctx, &sources, query, itemID)
	return sources, err
}

func (s *PostgresStore) GetSupplier(ctx context.Context, id string) (core.Supplier, error) {
	var supplier core.Supplier
	query := `SELECT * FROM suppliers WHERE id = $1`
	err := s.db.GetContext(ctx, &supplier, query, id)
	return supplier, err
}

func (s *PostgresStore) GetItemBySource(ctx context.Context, supplierID, productID string) (string, error) {
	var itemID string
	query := `SELECT item_id FROM item_sources WHERE supplier_id = $1 AND product_id = $2`
	err := s.db.GetContext(ctx, &itemID, query, supplierID, productID)
	return itemID, err
}
