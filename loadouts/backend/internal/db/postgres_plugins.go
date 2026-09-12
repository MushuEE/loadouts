package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// Postgres plugin persistence. It mirrors MemoryStore semantics in memory_plugins.go so
// the two backends stay interchangeable.

// --- Plugins ---

func (s *PostgresStore) CreatePlugin(ctx context.Context, p core.Plugin) error {
	query := `INSERT INTO plugins (id, slug, name, description, owner_type, owner_id, latest_version, is_public)
	          VALUES (:id, :slug, :name, :description, :owner_type, :owner_id, :latest_version, :is_public)`
	_, err := s.db.NamedExecContext(ctx, query, p)
	return err
}

func (s *PostgresStore) UpdatePlugin(ctx context.Context, p core.Plugin) error {
	query := `UPDATE plugins SET name = :name, description = :description,
	          latest_version = :latest_version, is_public = :is_public, updated_at = NOW()
	          WHERE id = :id`
	_, err := s.db.NamedExecContext(ctx, query, p)
	return err
}

func (s *PostgresStore) GetPlugin(ctx context.Context, id string) (core.Plugin, error) {
	var p core.Plugin
	err := s.db.GetContext(ctx, &p, `SELECT * FROM plugins WHERE id = $1`, id)
	return p, err
}

func (s *PostgresStore) GetPluginBySlug(ctx context.Context, slug string) (core.Plugin, error) {
	var p core.Plugin
	err := s.db.GetContext(ctx, &p, `SELECT * FROM plugins WHERE LOWER(slug) = LOWER($1)`, slug)
	return p, err
}

func (s *PostgresStore) ListPlugins(ctx context.Context, q core.PluginQuery) ([]core.Plugin, error) {
	plugins := []core.Plugin{}
	clauses := []string{"1 = 1"}
	args := []interface{}{}
	add := func(clause string, value interface{}) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	if q.OwnerType != "" {
		add("p.owner_type = $%d", string(q.OwnerType))
	}
	if q.OwnerID != "" {
		add("p.owner_id = $%d", q.OwnerID)
	}
	if q.Search != "" {
		args = append(args, "%"+strings.ToLower(q.Search)+"%")
		clauses = append(clauses, fmt.Sprintf(
			"(LOWER(p.name) LIKE $%d OR LOWER(p.description) LIKE $%d OR LOWER(p.slug) LIKE $%d)",
			len(args), len(args), len(args)))
	}
	if q.PublicOnly {
		// A private plugin stays visible to the profile that owns it.
		if q.IncludeMine != "" {
			args = append(args, q.IncludeMine)
			clauses = append(clauses, fmt.Sprintf(
				"(p.is_public OR (p.owner_type = 'profile' AND p.owner_id = $%d))", len(args)))
		} else {
			clauses = append(clauses, "p.is_public")
		}
	}
	if q.Surface != "" {
		// Surface lives inside the latest version's manifest, so the filter has to
		// reach into the JSONB rather than into a column.
		args = append(args, string(q.Surface))
		clauses = append(clauses, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM plugin_versions pv,
			     LATERAL jsonb_array_elements(pv.manifest->'views') AS view
			WHERE pv.plugin_id = p.id AND pv.version = p.latest_version
			  AND view->>'surface' = $%d)`, len(args)))
	}

	query := `SELECT p.* FROM plugins p WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY p.name`
	err := s.db.SelectContext(ctx, &plugins, query, args...)
	return plugins, err
}

func (s *PostgresStore) CreatePluginVersion(ctx context.Context, v core.PluginVersion) error {
	// The primary key on (plugin_id, version) is what makes versions immutable: a
	// republish of an existing version is a conflict, not an overwrite.
	query := `INSERT INTO plugin_versions (plugin_id, version, manifest, changelog, created_by)
	          VALUES (:plugin_id, :version, :manifest, :changelog, :created_by)`
	_, err := s.db.NamedExecContext(ctx, query, v)
	return err
}

func (s *PostgresStore) GetPluginVersion(ctx context.Context, pluginID string, version int) (core.PluginVersion, error) {
	var v core.PluginVersion
	err := s.db.GetContext(ctx, &v,
		`SELECT * FROM plugin_versions WHERE plugin_id = $1 AND version = $2`, pluginID, version)
	return v, err
}

func (s *PostgresStore) ListPluginVersions(ctx context.Context, pluginID string) ([]core.PluginVersion, error) {
	versions := []core.PluginVersion{}
	err := s.db.SelectContext(ctx, &versions,
		`SELECT * FROM plugin_versions WHERE plugin_id = $1 ORDER BY version`, pluginID)
	return versions, err
}

// --- Installs ---

func (s *PostgresStore) UpsertPluginInstall(ctx context.Context, install core.PluginInstall) error {
	// The unique index on (scope_type, scope_id, plugin_id) means re-installing upgrades
	// in place rather than stacking duplicates on one scope.
	query := `INSERT INTO plugin_installs
	            (id, plugin_id, version, scope_type, scope_id, granted_caps, settings, enabled, installed_by)
	          VALUES
	            (:id, :plugin_id, :version, :scope_type, :scope_id, :granted_caps, :settings, :enabled, :installed_by)
	          ON CONFLICT (scope_type, scope_id, plugin_id) DO UPDATE SET
	            version = EXCLUDED.version,
	            granted_caps = EXCLUDED.granted_caps,
	            settings = EXCLUDED.settings,
	            enabled = EXCLUDED.enabled,
	            updated_at = NOW()`
	_, err := s.db.NamedExecContext(ctx, query, install)
	return err
}

func (s *PostgresStore) DeletePluginInstall(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plugin_installs WHERE id = $1`, id)
	return err
}

func (s *PostgresStore) GetPluginInstall(ctx context.Context, id string) (core.PluginInstall, error) {
	var i core.PluginInstall
	err := s.db.GetContext(ctx, &i, `SELECT * FROM plugin_installs WHERE id = $1`, id)
	return i, err
}

func (s *PostgresStore) FindPluginInstall(ctx context.Context, scopeType, scopeID, pluginID string) (*core.PluginInstall, error) {
	var i core.PluginInstall
	err := s.db.GetContext(ctx, &i,
		`SELECT * FROM plugin_installs WHERE scope_type = $1 AND scope_id = $2 AND plugin_id = $3`,
		scopeType, scopeID, pluginID)
	if errors.Is(err, sql.ErrNoRows) {
		// Not installed is an ordinary answer, not a failure.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *PostgresStore) ListPluginInstalls(ctx context.Context, scopeType, scopeID string) ([]core.PluginInstall, error) {
	installs := []core.PluginInstall{}
	err := s.db.SelectContext(ctx, &installs,
		`SELECT * FROM plugin_installs
		 WHERE ($1 = '' OR scope_type = $1) AND ($2 = '' OR scope_id = $2)
		 ORDER BY created_at`, scopeType, scopeID)
	return installs, err
}

// --- Plugin storage ---

func (s *PostgresStore) PutPluginDatum(ctx context.Context, d core.PluginDatum) error {
	query := `INSERT INTO plugin_data (plugin_id, scope_type, scope_id, key, value, updated_by)
	          VALUES (:plugin_id, :scope_type, :scope_id, :key, :value, :updated_by)
	          ON CONFLICT (plugin_id, scope_type, scope_id, key) DO UPDATE SET
	            value = EXCLUDED.value,
	            updated_by = EXCLUDED.updated_by,
	            updated_at = NOW()`
	_, err := s.db.NamedExecContext(ctx, query, d)
	return err
}

func (s *PostgresStore) GetPluginDatum(ctx context.Context, pluginID, scopeType, scopeID, dataKey string) (*core.PluginDatum, error) {
	var d core.PluginDatum
	err := s.db.GetContext(ctx, &d,
		`SELECT * FROM plugin_data WHERE plugin_id = $1 AND scope_type = $2 AND scope_id = $3 AND key = $4`,
		pluginID, scopeType, scopeID, dataKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *PostgresStore) ListPluginData(ctx context.Context, pluginID, scopeType, scopeID string) ([]core.PluginDatum, error) {
	data := []core.PluginDatum{}
	err := s.db.SelectContext(ctx, &data,
		`SELECT * FROM plugin_data WHERE plugin_id = $1 AND scope_type = $2 AND scope_id = $3 ORDER BY key`,
		pluginID, scopeType, scopeID)
	return data, err
}

func (s *PostgresStore) DeletePluginDatum(ctx context.Context, pluginID, scopeType, scopeID, dataKey string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM plugin_data WHERE plugin_id = $1 AND scope_type = $2 AND scope_id = $3 AND key = $4`,
		pluginID, scopeType, scopeID, dataKey)
	return err
}
