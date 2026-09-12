package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// This file implements the Day 0 object model (identity, communities, layers, templates,
// loadouts) against Postgres. It intentionally mirrors the MemoryStore semantics so the two
// backends are interchangeable.

// --- Identity ---

func (s *PostgresStore) CreateUser(ctx context.Context, user core.User) error {
	query := `INSERT INTO users (id, email, display_name) VALUES (:id, :email, :display_name)`
	_, err := s.db.NamedExecContext(ctx, query, user)
	return err
}

func (s *PostgresStore) GetUser(ctx context.Context, id string) (core.User, error) {
	var user core.User
	err := s.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = $1`, id)
	return user, err
}

func (s *PostgresStore) ListUsers(ctx context.Context) ([]core.User, error) {
	users := []core.User{}
	err := s.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY created_at`)
	return users, err
}

func (s *PostgresStore) CreateProfile(ctx context.Context, profile core.Profile) error {
	query := `INSERT INTO profiles (id, user_id, handle, display_name, bio, avatar_url, is_sponsor)
	          VALUES (:id, :user_id, :handle, :display_name, :bio, :avatar_url, :is_sponsor)`
	_, err := s.db.NamedExecContext(ctx, query, profile)
	return err
}

func (s *PostgresStore) GetProfile(ctx context.Context, id string) (core.Profile, error) {
	var p core.Profile
	err := s.db.GetContext(ctx, &p, `SELECT * FROM profiles WHERE id = $1`, id)
	return p, err
}

func (s *PostgresStore) GetProfileByHandle(ctx context.Context, handle string) (core.Profile, error) {
	var p core.Profile
	err := s.db.GetContext(ctx, &p, `SELECT * FROM profiles WHERE LOWER(handle) = LOWER($1)`, handle)
	return p, err
}

func (s *PostgresStore) ListProfiles(ctx context.Context, userID string) ([]core.Profile, error) {
	profiles := []core.Profile{}
	if userID == "" {
		err := s.db.SelectContext(ctx, &profiles, `SELECT * FROM profiles ORDER BY handle`)
		return profiles, err
	}
	err := s.db.SelectContext(ctx, &profiles, `SELECT * FROM profiles WHERE user_id = $1 ORDER BY handle`, userID)
	return profiles, err
}

// --- Communities ---

func (s *PostgresStore) CreateCommunity(ctx context.Context, c core.Community) error {
	query := `INSERT INTO communities (id, slug, name, description, created_by, member_count, metadata_hint)
	          VALUES (:id, :slug, :name, :description, :created_by, :member_count, :metadata_hint)`
	_, err := s.db.NamedExecContext(ctx, query, c)
	return err
}

func (s *PostgresStore) UpdateCommunity(ctx context.Context, c core.Community) error {
	query := `UPDATE communities SET name = :name, description = :description,
	          metadata_hint = :metadata_hint, member_count = :member_count WHERE id = :id`
	_, err := s.db.NamedExecContext(ctx, query, c)
	return err
}

func (s *PostgresStore) GetCommunity(ctx context.Context, id string) (core.Community, error) {
	var c core.Community
	err := s.db.GetContext(ctx, &c, `SELECT * FROM communities WHERE id = $1`, id)
	return c, err
}

func (s *PostgresStore) GetCommunityBySlug(ctx context.Context, slug string) (core.Community, error) {
	var c core.Community
	err := s.db.GetContext(ctx, &c, `SELECT * FROM communities WHERE LOWER(slug) = LOWER($1)`, slug)
	return c, err
}

func (s *PostgresStore) ListCommunities(ctx context.Context, query string) ([]core.Community, error) {
	communities := []core.Community{}
	err := s.db.SelectContext(ctx, &communities,
		`SELECT * FROM communities WHERE name ILIKE $1 OR slug ILIKE $1 ORDER BY member_count DESC, name`,
		"%"+query+"%")
	return communities, err
}

func (s *PostgresStore) UpsertMembership(ctx context.Context, m core.CommunityMembership) error {
	query := `INSERT INTO community_members (community_id, profile_id, role)
	          VALUES (:community_id, :profile_id, :role)
	          ON CONFLICT (community_id, profile_id) DO UPDATE SET role = EXCLUDED.role`
	if _, err := s.db.NamedExecContext(ctx, query, m); err != nil {
		return err
	}
	return s.syncMemberCount(ctx, m.CommunityID)
}

func (s *PostgresStore) DeleteMembership(ctx context.Context, communityID, profileID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM community_members WHERE community_id = $1 AND profile_id = $2`, communityID, profileID)
	if err != nil {
		return err
	}
	return s.syncMemberCount(ctx, communityID)
}

func (s *PostgresStore) syncMemberCount(ctx context.Context, communityID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE communities SET member_count =
		   (SELECT COUNT(*) FROM community_members WHERE community_id = $1) WHERE id = $1`, communityID)
	return err
}

func (s *PostgresStore) GetMembership(ctx context.Context, communityID, profileID string) (*core.CommunityMembership, error) {
	var m core.CommunityMembership
	err := s.db.GetContext(ctx, &m,
		`SELECT * FROM community_members WHERE community_id = $1 AND profile_id = $2`, communityID, profileID)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *PostgresStore) ListMemberships(ctx context.Context, communityID, profileID string) ([]core.CommunityMembership, error) {
	members := []core.CommunityMembership{}
	query := `SELECT * FROM community_members WHERE ($1 = '' OR community_id = $1) AND ($2 = '' OR profile_id = $2) ORDER BY joined_at`
	err := s.db.SelectContext(ctx, &members, query, communityID, profileID)
	return members, err
}

// --- Metadata layers ---

func (s *PostgresStore) UpsertCommunityItemLayer(ctx context.Context, layer core.CommunityItemLayer) error {
	query := `INSERT INTO community_item_layers (community_id, item_id, metadata, updated_by, updated_at)
	          VALUES (:community_id, :item_id, :metadata, :updated_by, CURRENT_TIMESTAMP)
	          ON CONFLICT (community_id, item_id) DO UPDATE SET
	            metadata = EXCLUDED.metadata,
	            updated_by = EXCLUDED.updated_by,
	            updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.NamedExecContext(ctx, query, layer)
	return err
}

func (s *PostgresStore) GetCommunityItemLayer(ctx context.Context, communityID, itemID string) (*core.CommunityItemLayer, error) {
	var layer core.CommunityItemLayer
	err := s.db.GetContext(ctx, &layer,
		`SELECT * FROM community_item_layers WHERE community_id = $1 AND item_id = $2`, communityID, itemID)
	if err != nil {
		return nil, err
	}
	return &layer, nil
}

func (s *PostgresStore) UpsertProfileItemLayer(ctx context.Context, layer core.ProfileItemLayer) error {
	query := `INSERT INTO profile_item_layers (profile_id, item_id, custom_image_url, public_metadata, private_metadata, updated_at)
	          VALUES (:profile_id, :item_id, :custom_image_url, :public_metadata, :private_metadata, CURRENT_TIMESTAMP)
	          ON CONFLICT (profile_id, item_id) DO UPDATE SET
	            custom_image_url = EXCLUDED.custom_image_url,
	            public_metadata = EXCLUDED.public_metadata,
	            private_metadata = EXCLUDED.private_metadata,
	            updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.NamedExecContext(ctx, query, layer)
	return err
}

func (s *PostgresStore) GetProfileItemLayer(ctx context.Context, profileID, itemID string) (*core.ProfileItemLayer, error) {
	var layer core.ProfileItemLayer
	err := s.db.GetContext(ctx, &layer,
		`SELECT * FROM profile_item_layers WHERE profile_id = $1 AND item_id = $2`, profileID, itemID)
	if err != nil {
		return nil, err
	}
	return &layer, nil
}

// --- Templates ---

func (s *PostgresStore) CreateTemplate(ctx context.Context, t core.Template) error {
	query := `INSERT INTO templates (id, name, description, owner_type, owner_id, community_id, latest_version, is_public)
	          VALUES (:id, :name, :description, :owner_type, :owner_id, :community_id, :latest_version, :is_public)`
	_, err := s.db.NamedExecContext(ctx, query, t)
	return err
}

func (s *PostgresStore) UpdateTemplate(ctx context.Context, t core.Template) error {
	query := `UPDATE templates SET name = :name, description = :description, is_public = :is_public,
	          latest_version = :latest_version, updated_at = CURRENT_TIMESTAMP WHERE id = :id`
	_, err := s.db.NamedExecContext(ctx, query, t)
	return err
}

func (s *PostgresStore) GetTemplate(ctx context.Context, id string) (core.Template, error) {
	var t core.Template
	err := s.db.GetContext(ctx, &t, `SELECT * FROM templates WHERE id = $1`, id)
	return t, err
}

func (s *PostgresStore) ListTemplates(ctx context.Context, q core.TemplateQuery) ([]core.Template, error) {
	templates := []core.Template{}
	clauses := []string{"(name ILIKE :text OR description ILIKE :text)"}
	args := map[string]interface{}{"text": "%" + q.Text + "%"}

	if q.OwnerType != "" {
		clauses = append(clauses, "owner_type = :owner_type")
		args["owner_type"] = string(q.OwnerType)
	}
	if q.OwnerID != "" {
		clauses = append(clauses, "owner_id = :owner_id")
		args["owner_id"] = q.OwnerID
	}
	if q.CommunityID != "" {
		clauses = append(clauses, "community_id = :community_id")
		args["community_id"] = q.CommunityID
	}
	if q.OnlyPublic {
		clauses = append(clauses, "is_public = TRUE")
	}

	query := "SELECT * FROM templates WHERE " + strings.Join(clauses, " AND ") + " ORDER BY name"
	rows, err := s.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t core.Template
		if err := rows.StructScan(&t); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (s *PostgresStore) CreateTemplateVersion(ctx context.Context, v core.TemplateVersion) error {
	// Versions are immutable: a conflicting insert is an error, never an update.
	query := `INSERT INTO template_versions (template_id, version, slots, changelog)
	          VALUES (:template_id, :version, :slots, :changelog)`
	_, err := s.db.NamedExecContext(ctx, query, v)
	return err
}

func (s *PostgresStore) GetTemplateVersion(ctx context.Context, templateID string, version int) (core.TemplateVersion, error) {
	var v core.TemplateVersion
	err := s.db.GetContext(ctx, &v,
		`SELECT * FROM template_versions WHERE template_id = $1 AND version = $2`, templateID, version)
	return v, err
}

func (s *PostgresStore) ListTemplateVersions(ctx context.Context, templateID string) ([]core.TemplateVersion, error) {
	versions := []core.TemplateVersion{}
	err := s.db.SelectContext(ctx, &versions,
		`SELECT * FROM template_versions WHERE template_id = $1 ORDER BY version`, templateID)
	return versions, err
}

// --- Loadouts ---

func (s *PostgresStore) CreateLoadout(ctx context.Context, l core.Loadout) error {
	query := `INSERT INTO loadouts (id, name, description, owner_profile_id, community_id, template_id,
	            template_version, visibility, status, forked_from, cover_image_url, fork_count)
	          VALUES (:id, :name, :description, :owner_profile_id, :community_id, :template_id,
	            :template_version, :visibility, :status, :forked_from, :cover_image_url, :fork_count)`
	_, err := s.db.NamedExecContext(ctx, query, l)
	return err
}

func (s *PostgresStore) UpdateLoadout(ctx context.Context, l core.Loadout) error {
	query := `UPDATE loadouts SET name = :name, description = :description, community_id = :community_id,
	            template_id = :template_id, template_version = :template_version, visibility = :visibility,
	            status = :status, cover_image_url = :cover_image_url, fork_count = :fork_count,
	            updated_at = CURRENT_TIMESTAMP
	          WHERE id = :id`
	_, err := s.db.NamedExecContext(ctx, query, l)
	return err
}

func (s *PostgresStore) DeleteLoadout(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM loadouts WHERE id = $1`, id)
	return err
}

func (s *PostgresStore) GetLoadout(ctx context.Context, id string) (core.Loadout, error) {
	var l core.Loadout
	err := s.db.GetContext(ctx, &l, `SELECT * FROM loadouts WHERE id = $1`, id)
	return l, err
}

func (s *PostgresStore) ListLoadouts(ctx context.Context, q core.DiscoverQuery) ([]core.Loadout, error) {
	loadouts := []core.Loadout{}
	clauses := []string{"(name ILIKE :text OR description ILIKE :text)"}
	args := map[string]interface{}{"text": "%" + q.Text + "%"}

	if q.ProfileID != "" {
		clauses = append(clauses, "owner_profile_id = :profile_id")
		args["profile_id"] = q.ProfileID
	}
	if q.CommunityID != "" {
		clauses = append(clauses, "community_id = :community_id")
		args["community_id"] = q.CommunityID
	}
	if q.TemplateID != "" {
		clauses = append(clauses, "template_id = :template_id")
		args["template_id"] = q.TemplateID
	}
	if q.Status != "" {
		clauses = append(clauses, "status = :status")
		args["status"] = string(q.Status)
	}
	if q.OnlyPublic {
		clauses = append(clauses, "visibility = 'public'")
	}

	query := "SELECT * FROM loadouts WHERE " + strings.Join(clauses, " AND ") + " ORDER BY updated_at DESC"
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l core.Loadout
		if err := rows.StructScan(&l); err != nil {
			return nil, err
		}
		loadouts = append(loadouts, l)
	}
	return loadouts, rows.Err()
}

// ReplaceLoadoutEntries swaps the entry set atomically; the client always sends the whole tree.
func (s *PostgresStore) ReplaceLoadoutEntries(ctx context.Context, loadoutID string, entries []core.LoadoutEntry) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, `DELETE FROM loadout_entries WHERE loadout_id = $1`, loadoutID); err != nil {
		return err
	}

	query := `INSERT INTO loadout_entries (id, loadout_id, slot_id, parent_entry_id, item_id, quantity, note, position)
	          VALUES (:id, :loadout_id, :slot_id, :parent_entry_id, :item_id, :quantity, :note, :position)`
	for _, e := range entries {
		e.LoadoutID = loadoutID
		if _, err := tx.NamedExecContext(ctx, query, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) ListLoadoutEntries(ctx context.Context, loadoutID string) ([]core.LoadoutEntry, error) {
	entries := []core.LoadoutEntry{}
	err := s.db.SelectContext(ctx, &entries,
		`SELECT * FROM loadout_entries WHERE loadout_id = $1 ORDER BY position`, loadoutID)
	return entries, err
}
