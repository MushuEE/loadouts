package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// Favorites against Postgres. Mirrors MemoryStore semantics exactly, including the two
// behaviours that are easy to get subtly different:
//
//   - Favoriting something already favorited is a re-confirmation of the same row.
//   - CreatedAt survives a re-confirmation; only ConfirmedAt moves.

func (s *PostgresStore) UpsertFavorite(ctx context.Context, f core.Favorite) error {
	// ON CONFLICT deliberately leaves id and created_at alone: the endorsement dates from
	// when it was first given, not from the most recent re-confirmation.
	query := `INSERT INTO favorites
	            (id, scope_type, scope_id, loadout_id, note, fingerprint, actor_profile_id, created_at, confirmed_at)
	          VALUES
	            (:id, :scope_type, :scope_id, :loadout_id, :note, :fingerprint, :actor_profile_id, :created_at, :confirmed_at)
	          ON CONFLICT (scope_type, scope_id, loadout_id) DO UPDATE SET
	            note = EXCLUDED.note,
	            fingerprint = EXCLUDED.fingerprint,
	            actor_profile_id = EXCLUDED.actor_profile_id,
	            confirmed_at = EXCLUDED.confirmed_at`
	_, err := s.db.NamedExecContext(ctx, query, f)
	return err
}

func (s *PostgresStore) DeleteFavorite(ctx context.Context, scopeType core.FavoriteScope, scopeID, loadoutID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM favorites WHERE scope_type = $1 AND scope_id = $2 AND loadout_id = $3`,
		string(scopeType), scopeID, loadoutID)
	return err
}

func (s *PostgresStore) GetFavorite(ctx context.Context, scopeType core.FavoriteScope, scopeID, loadoutID string) (*core.Favorite, error) {
	var f core.Favorite
	err := s.db.GetContext(ctx, &f,
		`SELECT * FROM favorites WHERE scope_type = $1 AND scope_id = $2 AND loadout_id = $3`,
		string(scopeType), scopeID, loadoutID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // Not favorited is a normal answer, not an error.
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *PostgresStore) ListFavoritesForScope(ctx context.Context, scopeType core.FavoriteScope, scopeID string) ([]core.Favorite, error) {
	favorites := []core.Favorite{}
	err := s.db.SelectContext(ctx, &favorites,
		`SELECT * FROM favorites WHERE scope_type = $1 AND scope_id = $2 ORDER BY confirmed_at DESC`,
		string(scopeType), scopeID)
	return favorites, err
}

func (s *PostgresStore) ListFavoritesForLoadout(ctx context.Context, loadoutID string) ([]core.Favorite, error) {
	favorites := []core.Favorite{}
	err := s.db.SelectContext(ctx, &favorites,
		`SELECT * FROM favorites WHERE loadout_id = $1 ORDER BY confirmed_at DESC`, loadoutID)
	return favorites, err
}
