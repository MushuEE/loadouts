package db

import (
	"context"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/jmoiron/sqlx"
)

// Tags against Postgres. Mirrors MemoryStore: a set replaces the profile's whole list for
// the item, and counts are distinct profiles, most used first.

func (s *PostgresStore) SetProfileItemTags(ctx context.Context, profileID, itemID string, tags []string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_item_tags WHERE profile_id = $1 AND item_id = $2`, profileID, itemID); err != nil {
		return err
	}
	for i, t := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_item_tags (profile_id, item_id, tag, position) VALUES ($1, $2, $3, $4)`,
			profileID, itemID, t, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) GetProfileItemTags(ctx context.Context, profileID, itemID string) ([]string, error) {
	tags := []string{}
	err := s.db.SelectContext(ctx, &tags,
		`SELECT tag FROM profile_item_tags WHERE profile_id = $1 AND item_id = $2 ORDER BY position`,
		profileID, itemID)
	return tags, err
}

func (s *PostgresStore) CountItemTags(ctx context.Context, itemID string) ([]core.TagCount, error) {
	counts := []core.TagCount{}
	err := s.db.SelectContext(ctx, &counts,
		`SELECT tag, COUNT(*) AS count FROM profile_item_tags WHERE item_id = $1
		 GROUP BY tag ORDER BY count DESC, tag`, itemID)
	return counts, err
}

func (s *PostgresStore) CountTags(ctx context.Context, prefix string, limit int) ([]core.TagCount, error) {
	if limit <= 0 {
		limit = 1000
	}
	counts := []core.TagCount{}
	err := s.db.SelectContext(ctx, &counts,
		`SELECT tag, COUNT(DISTINCT profile_id) AS count FROM profile_item_tags WHERE left(tag, length($1)) = $1
		 GROUP BY tag ORDER BY count DESC, tag LIMIT $2`, prefix, limit)
	return counts, err
}

func (s *PostgresStore) ProfileTagsForItems(ctx context.Context, profileID string, itemIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(
		`SELECT item_id, tag FROM profile_item_tags WHERE profile_id = ? AND item_id IN (?) ORDER BY item_id, position`,
		profileID, itemIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ItemID string `db:"item_id"`
		Tag    string `db:"tag"`
	}
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ItemID] = append(out[r.ItemID], r.Tag)
	}
	return out, nil
}

func (s *PostgresStore) CountTagsForItems(ctx context.Context, itemIDs []string) (map[string][]core.TagCount, error) {
	out := map[string][]core.TagCount{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(
		`SELECT item_id, tag, COUNT(*) AS count FROM profile_item_tags WHERE item_id IN (?)
		 GROUP BY item_id, tag ORDER BY item_id, count DESC, tag`, itemIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ItemID string `db:"item_id"`
		Tag    string `db:"tag"`
		Count  int    `db:"count"`
	}
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ItemID] = append(out[r.ItemID], core.TagCount{Tag: r.Tag, Count: r.Count})
	}
	return out, nil
}

func (s *PostgresStore) ProfileItemIDs(ctx context.Context, profileID string) ([]string, error) {
	ids := []string{}
	err := s.db.SelectContext(ctx, &ids, `
		SELECT item_id FROM profile_item_tags WHERE profile_id = $1
		UNION SELECT item_id FROM profile_item_layers WHERE profile_id = $1
		UNION SELECT e.item_id FROM loadout_entries e JOIN loadouts l ON l.id = e.loadout_id
		      WHERE l.owner_profile_id = $1 AND e.item_id IS NOT NULL AND e.item_id <> ''
		UNION SELECT id FROM items WHERE imported_by = $1
		ORDER BY 1`, profileID)
	return ids, err
}
