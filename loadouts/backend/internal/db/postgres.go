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
	query := `INSERT INTO items (id, name, base_metadata)
	          VALUES (:id, :name, :base_metadata)`
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
	var items []core.Item
	// Very basic search for now. In the future, this could use GIN index or ElasticSearch.
	query := `SELECT * FROM items WHERE name ILIKE $1 OR id ILIKE $1`
	err := s.db.SelectContext(ctx, &items, query, "%"+searchTerm+"%")
	return items, err
}

func (s *PostgresStore) UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error {
	query := `INSERT INTO user_metadata (user_id, item_id, overrides, open_data, updated_at)
	          VALUES (:user_id, :item_id, :overrides, :open_data, CURRENT_TIMESTAMP)
	          ON CONFLICT (user_id, item_id) DO UPDATE SET
	          overrides = EXCLUDED.overrides,
	          open_data = EXCLUDED.open_data,
	          updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.NamedExecContext(ctx, query, meta)
	return err
}

func (s *PostgresStore) GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error) {
	var meta core.UserMetadata
	query := `SELECT * FROM user_metadata WHERE user_id = $1 AND item_id = $2`
	err := s.db.GetContext(ctx, &meta, query, userID, itemID)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}
