package db

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// In-memory plugin persistence. It mirrors the Postgres semantics in
// postgres_plugins.go exactly, so tests that run against MemoryStore are meaningful.

// --- Plugins ---

func (s *MemoryStore) CreatePlugin(ctx context.Context, p core.Plugin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.plugins[p.ID]; exists {
		return fmt.Errorf("plugin %s already exists", p.ID)
	}
	for _, existing := range s.plugins {
		if existing.Slug == p.Slug {
			return fmt.Errorf("plugin slug %q is taken", p.Slug)
		}
	}
	s.plugins[p.ID] = p
	return nil
}

func (s *MemoryStore) UpdatePlugin(ctx context.Context, p core.Plugin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.plugins[p.ID]; !exists {
		return fmt.Errorf("plugin not found")
	}
	p.UpdatedAt = time.Now().UTC()
	s.plugins[p.ID] = p
	return nil
}

func (s *MemoryStore) GetPlugin(ctx context.Context, id string) (core.Plugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.plugins[id]
	if !ok {
		return core.Plugin{}, fmt.Errorf("plugin not found")
	}
	return p, nil
}

func (s *MemoryStore) GetPluginBySlug(ctx context.Context, slug string) (core.Plugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.plugins {
		if p.Slug == slug {
			return p, nil
		}
	}
	return core.Plugin{}, fmt.Errorf("plugin not found")
}

func (s *MemoryStore) ListPlugins(ctx context.Context, q core.PluginQuery) ([]core.Plugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Plugin, 0)
	for _, p := range s.plugins {
		if q.OwnerType != "" && p.OwnerType != q.OwnerType {
			continue
		}
		if q.OwnerID != "" && p.OwnerID != q.OwnerID {
			continue
		}
		// A private plugin is still visible to the profile that owns it, which is what
		// makes "publish, then look at it before sharing" work.
		if q.PublicOnly && !p.IsPublic && !s.ownedByLocked(p, q.IncludeMine) {
			continue
		}
		if !contains(p.Name, q.Search) && !contains(p.Description, q.Search) && !contains(p.Slug, q.Search) {
			continue
		}
		if q.Surface != "" && !s.latestVersionHasSurfaceLocked(p, q.Surface) {
			continue
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (s *MemoryStore) ownedByLocked(p core.Plugin, profileID string) bool {
	return profileID != "" && p.OwnerType == core.OwnerProfile && p.OwnerID == profileID
}

func (s *MemoryStore) latestVersionHasSurfaceLocked(p core.Plugin, surface core.Surface) bool {
	v, ok := s.pluginVers[key(p.ID, fmt.Sprint(p.LatestVersion))]
	if !ok {
		return false
	}
	for _, view := range v.Manifest.Views {
		if view.Surface == surface {
			return true
		}
	}
	return false
}

func (s *MemoryStore) CreatePluginVersion(ctx context.Context, v core.PluginVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(v.PluginID, fmt.Sprint(v.Version))
	// Versions are immutable so that an install pinned to v1 keeps behaving exactly as
	// it did when the installer approved it.
	if _, exists := s.pluginVers[k]; exists {
		return fmt.Errorf("plugin version %s v%d already exists (versions are immutable)", v.PluginID, v.Version)
	}
	s.pluginVers[k] = v
	return nil
}

func (s *MemoryStore) GetPluginVersion(ctx context.Context, pluginID string, version int) (core.PluginVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.pluginVers[key(pluginID, fmt.Sprint(version))]
	if !ok {
		return core.PluginVersion{}, fmt.Errorf("plugin version not found")
	}
	return v, nil
}

func (s *MemoryStore) ListPluginVersions(ctx context.Context, pluginID string) ([]core.PluginVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.PluginVersion, 0)
	for _, v := range s.pluginVers {
		if v.PluginID == pluginID {
			list = append(list, v)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

// --- Installs ---

func (s *MemoryStore) UpsertPluginInstall(ctx context.Context, install core.PluginInstall) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// One install per (scope, plugin): re-installing upgrades in place rather than
	// stacking duplicate copies of the same plugin on one profile.
	for id, existing := range s.installs {
		if existing.ScopeType == install.ScopeType &&
			existing.ScopeID == install.ScopeID &&
			existing.PluginID == install.PluginID &&
			id != install.ID {
			delete(s.installs, id)
		}
	}
	install.UpdatedAt = time.Now().UTC()
	if install.CreatedAt.IsZero() {
		install.CreatedAt = install.UpdatedAt
	}
	s.installs[install.ID] = install
	return nil
}

func (s *MemoryStore) DeletePluginInstall(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.installs[id]; !ok {
		return fmt.Errorf("install not found")
	}
	delete(s.installs, id)
	return nil
}

func (s *MemoryStore) GetPluginInstall(ctx context.Context, id string) (core.PluginInstall, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.installs[id]
	if !ok {
		return core.PluginInstall{}, fmt.Errorf("install not found")
	}
	return i, nil
}

func (s *MemoryStore) FindPluginInstall(ctx context.Context, scopeType, scopeID, pluginID string) (*core.PluginInstall, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, i := range s.installs {
		if i.ScopeType == scopeType && i.ScopeID == scopeID && i.PluginID == pluginID {
			found := i
			return &found, nil
		}
	}
	return nil, nil
}

func (s *MemoryStore) ListPluginInstalls(ctx context.Context, scopeType, scopeID string) ([]core.PluginInstall, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.PluginInstall, 0)
	for _, i := range s.installs {
		if scopeType != "" && i.ScopeType != scopeType {
			continue
		}
		if scopeID != "" && i.ScopeID != scopeID {
			continue
		}
		list = append(list, i)
	}
	sort.Slice(list, func(a, b int) bool { return list[a].CreatedAt.Before(list[b].CreatedAt) })
	return list, nil
}

// --- Plugin storage ---

func (s *MemoryStore) PutPluginDatum(ctx context.Context, d core.PluginDatum) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d.UpdatedAt = time.Now().UTC()
	s.pluginData[key(d.PluginID, d.ScopeType, d.ScopeID, d.Key)] = d
	return nil
}

func (s *MemoryStore) GetPluginDatum(ctx context.Context, pluginID, scopeType, scopeID, dataKey string) (*core.PluginDatum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.pluginData[key(pluginID, scopeType, scopeID, dataKey)]
	if !ok {
		return nil, nil
	}
	found := d
	return &found, nil
}

func (s *MemoryStore) ListPluginData(ctx context.Context, pluginID, scopeType, scopeID string) ([]core.PluginDatum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.PluginDatum, 0)
	for _, d := range s.pluginData {
		if d.PluginID != pluginID || d.ScopeType != scopeType || d.ScopeID != scopeID {
			continue
		}
		list = append(list, d)
	}
	sort.Slice(list, func(a, b int) bool { return list[a].Key < list[b].Key })
	return list, nil
}

func (s *MemoryStore) DeletePluginDatum(ctx context.Context, pluginID, scopeType, scopeID, dataKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pluginData, key(pluginID, scopeType, scopeID, dataKey))
	return nil
}
