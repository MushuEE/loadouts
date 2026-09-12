package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// MemoryStore is the zero-dependency Day 0 store. It backs local development, tests, and
// the demo seed so the whole platform can run with `go run ./cmd/server`.
type MemoryStore struct {
	mu           sync.RWMutex
	schemas      map[string]core.SchemaDefinition
	items        map[string]core.Item
	suppliers    map[string]core.Supplier
	itemSources  map[string][]core.ItemSource
	users        map[string]core.User
	profiles     map[string]core.Profile
	communities  map[string]core.Community
	memberships  map[string]core.CommunityMembership // "communityID:profileID"
	commLayers   map[string]core.CommunityItemLayer  // "communityID:itemID"
	profLayers   map[string]core.ProfileItemLayer    // "profileID:itemID"
	templates    map[string]core.Template
	tmplVersions map[string]core.TemplateVersion // "templateID:version"
	loadouts     map[string]core.Loadout
	entries      map[string][]core.LoadoutEntry // loadoutID -> entries
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schemas: make(map[string]core.SchemaDefinition),
		items:   make(map[string]core.Item),
		suppliers: map[string]core.Supplier{
			"amazon": {ID: "amazon", Name: "Amazon", BaseURL: "https://www.amazon.com", AffiliateTemplate: "{{.BaseURL}}/dp/{{.ProductID}}?tag=loadouts-20"},
			"rei":    {ID: "rei", Name: "REI", BaseURL: "https://www.rei.com", AffiliateTemplate: "{{.BaseURL}}/product/{{.ProductID}}?cm_mmc=aff_AL-_-loadouts"},
		},
		itemSources:  make(map[string][]core.ItemSource),
		users:        make(map[string]core.User),
		profiles:     make(map[string]core.Profile),
		communities:  make(map[string]core.Community),
		memberships:  make(map[string]core.CommunityMembership),
		commLayers:   make(map[string]core.CommunityItemLayer),
		profLayers:   make(map[string]core.ProfileItemLayer),
		templates:    make(map[string]core.Template),
		tmplVersions: make(map[string]core.TemplateVersion),
		loadouts:     make(map[string]core.Loadout),
		entries:      make(map[string][]core.LoadoutEntry),
	}
}

func key(parts ...string) string { return strings.Join(parts, ":") }

func contains(haystack, needle string) bool {
	return needle == "" || strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// --- Schemas ---

func (s *MemoryStore) CreateSchema(ctx context.Context, schema core.SchemaDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemas[key(schema.ID, schema.Version)] = schema
	return nil
}

func (s *MemoryStore) GetSchema(ctx context.Context, id, version string) (core.SchemaDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	schema, ok := s.schemas[key(id, version)]
	if !ok {
		return core.SchemaDefinition{}, fmt.Errorf("schema not found")
	}
	return schema, nil
}

func (s *MemoryStore) ListSchemas(ctx context.Context) ([]core.SchemaDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.SchemaDefinition, 0, len(s.schemas))
	for _, v := range s.schemas {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

// --- Items ---

func (s *MemoryStore) CreateItem(ctx context.Context, item core.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *MemoryStore) GetItem(ctx context.Context, id string) (core.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return core.Item{}, fmt.Errorf("item not found")
	}
	return item, nil
}

func (s *MemoryStore) ListItems(ctx context.Context, query string) ([]core.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Item, 0)
	for _, v := range s.items {
		if contains(v.Name, query) || contains(v.Category, query) {
			list = append(list, v)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// --- User metadata (legacy compat over profile layers) ---

func (s *MemoryStore) UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error {
	return s.UpsertProfileItemLayer(ctx, core.ProfileItemLayer{
		ProfileID:       meta.UserID,
		ItemID:          meta.ItemID,
		CustomImageURL:  meta.CustomImageURL,
		PublicMetadata:  meta.Overrides,
		PrivateMetadata: meta.OpenData,
	})
}

func (s *MemoryStore) GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error) {
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

// --- Item sources ---

func (s *MemoryStore) GetItemSources(ctx context.Context, itemID string) ([]core.ItemSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.itemSources[itemID], nil
}

func (s *MemoryStore) GetSupplier(ctx context.Context, id string) (core.Supplier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sup, ok := s.suppliers[id]
	if !ok {
		return core.Supplier{}, fmt.Errorf("supplier not found")
	}
	return sup, nil
}

func (s *MemoryStore) GetItemBySource(ctx context.Context, supplierID, productID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for itemID, sources := range s.itemSources {
		for _, src := range sources {
			if src.SupplierID == supplierID && src.ProductID == productID {
				return itemID, nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}

// UpsertItemSource mirrors the Postgres UNIQUE(supplier_id, product_id) constraint:
// re-importing the same product URL refreshes the price instead of stacking duplicates.
func (s *MemoryStore) UpsertItemSource(ctx context.Context, source core.ItemSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if source.LastUpdated.IsZero() {
		source.LastUpdated = time.Now().UTC()
	}
	existing := s.itemSources[source.ItemID]
	for i, src := range existing {
		if src.SupplierID == source.SupplierID && src.ProductID == source.ProductID {
			if source.ID == "" {
				source.ID = src.ID
			}
			existing[i] = source
			s.itemSources[source.ItemID] = existing
			return nil
		}
	}
	if source.ID == "" {
		source.ID = core.NewID("src")
	}
	s.itemSources[source.ItemID] = append(existing, source)
	return nil
}

func (s *MemoryStore) UpsertSupplier(ctx context.Context, supplier core.Supplier) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suppliers[supplier.ID] = supplier
	return nil
}

func (s *MemoryStore) ListSuppliers(ctx context.Context) ([]core.Supplier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Supplier, 0, len(s.suppliers))
	for _, v := range s.suppliers {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (s *MemoryStore) AddItemSource(itemID string, src core.ItemSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.itemSources[itemID] = append(s.itemSources[itemID], src)
}

// --- Identity ---

func (s *MemoryStore) CreateUser(ctx context.Context, user core.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[user.ID]; exists {
		return fmt.Errorf("user %s already exists", user.ID)
	}
	s.users[user.ID] = user
	return nil
}

func (s *MemoryStore) GetUser(ctx context.Context, id string) (core.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return core.User{}, fmt.Errorf("user not found")
	}
	return u, nil
}

func (s *MemoryStore) ListUsers(ctx context.Context) ([]core.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

func (s *MemoryStore) CreateProfile(ctx context.Context, profile core.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.profiles {
		if strings.EqualFold(p.Handle, profile.Handle) {
			return fmt.Errorf("handle %s is taken", profile.Handle)
		}
	}
	s.profiles[profile.ID] = profile
	return nil
}

func (s *MemoryStore) GetProfile(ctx context.Context, id string) (core.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[id]
	if !ok {
		return core.Profile{}, fmt.Errorf("profile not found")
	}
	return p, nil
}

func (s *MemoryStore) GetProfileByHandle(ctx context.Context, handle string) (core.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.profiles {
		if strings.EqualFold(p.Handle, handle) {
			return p, nil
		}
	}
	return core.Profile{}, fmt.Errorf("profile not found")
}

func (s *MemoryStore) ListProfiles(ctx context.Context, userID string) ([]core.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Profile, 0)
	for _, p := range s.profiles {
		if userID == "" || p.UserID == userID {
			list = append(list, p)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Handle < list[j].Handle })
	return list, nil
}

// --- Communities ---

func (s *MemoryStore) CreateCommunity(ctx context.Context, community core.Community) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.communities {
		if strings.EqualFold(c.Slug, community.Slug) {
			return fmt.Errorf("community slug %s is taken", community.Slug)
		}
	}
	s.communities[community.ID] = community
	return nil
}

func (s *MemoryStore) UpdateCommunity(ctx context.Context, community core.Community) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.communities[community.ID]; !ok {
		return fmt.Errorf("community not found")
	}
	s.communities[community.ID] = community
	return nil
}

func (s *MemoryStore) GetCommunity(ctx context.Context, id string) (core.Community, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.communities[id]
	if !ok {
		return core.Community{}, fmt.Errorf("community not found")
	}
	return c, nil
}

func (s *MemoryStore) GetCommunityBySlug(ctx context.Context, slug string) (core.Community, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.communities {
		if strings.EqualFold(c.Slug, slug) {
			return c, nil
		}
	}
	return core.Community{}, fmt.Errorf("community not found")
}

func (s *MemoryStore) ListCommunities(ctx context.Context, query string) ([]core.Community, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Community, 0)
	for _, c := range s.communities {
		if contains(c.Name, query) || contains(c.Slug, query) {
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (s *MemoryStore) UpsertMembership(ctx context.Context, m core.CommunityMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberships[key(m.CommunityID, m.ProfileID)] = m
	s.recalcMemberCountLocked(m.CommunityID)
	return nil
}

func (s *MemoryStore) DeleteMembership(ctx context.Context, communityID, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memberships, key(communityID, profileID))
	s.recalcMemberCountLocked(communityID)
	return nil
}

// recalcMemberCountLocked keeps the denormalized counter honest. Callers must hold the lock.
func (s *MemoryStore) recalcMemberCountLocked(communityID string) {
	count := 0
	for _, m := range s.memberships {
		if m.CommunityID == communityID {
			count++
		}
	}
	if c, ok := s.communities[communityID]; ok {
		c.MemberCount = count
		s.communities[communityID] = c
	}
}

func (s *MemoryStore) GetMembership(ctx context.Context, communityID, profileID string) (*core.CommunityMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.memberships[key(communityID, profileID)]
	if !ok {
		return nil, fmt.Errorf("membership not found")
	}
	return &m, nil
}

func (s *MemoryStore) ListMemberships(ctx context.Context, communityID, profileID string) ([]core.CommunityMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.CommunityMembership, 0)
	for _, m := range s.memberships {
		if communityID != "" && m.CommunityID != communityID {
			continue
		}
		if profileID != "" && m.ProfileID != profileID {
			continue
		}
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].JoinedAt.Before(list[j].JoinedAt) })
	return list, nil
}

// --- Layers ---

func (s *MemoryStore) UpsertCommunityItemLayer(ctx context.Context, layer core.CommunityItemLayer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commLayers[key(layer.CommunityID, layer.ItemID)] = layer
	return nil
}

func (s *MemoryStore) GetCommunityItemLayer(ctx context.Context, communityID, itemID string) (*core.CommunityItemLayer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.commLayers[key(communityID, itemID)]
	if !ok {
		return nil, fmt.Errorf("community item layer not found")
	}
	return &l, nil
}

func (s *MemoryStore) UpsertProfileItemLayer(ctx context.Context, layer core.ProfileItemLayer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profLayers[key(layer.ProfileID, layer.ItemID)] = layer
	return nil
}

func (s *MemoryStore) GetProfileItemLayer(ctx context.Context, profileID, itemID string) (*core.ProfileItemLayer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.profLayers[key(profileID, itemID)]
	if !ok {
		return nil, fmt.Errorf("profile item layer not found")
	}
	return &l, nil
}

// --- Templates ---

func (s *MemoryStore) CreateTemplate(ctx context.Context, tmpl core.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.templates[tmpl.ID]; exists {
		return fmt.Errorf("template %s already exists", tmpl.ID)
	}
	s.templates[tmpl.ID] = tmpl
	return nil
}

func (s *MemoryStore) UpdateTemplate(ctx context.Context, tmpl core.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.templates[tmpl.ID]; !ok {
		return fmt.Errorf("template not found")
	}
	s.templates[tmpl.ID] = tmpl
	return nil
}

func (s *MemoryStore) GetTemplate(ctx context.Context, id string) (core.Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	if !ok {
		return core.Template{}, fmt.Errorf("template not found")
	}
	return t, nil
}

func (s *MemoryStore) ListTemplates(ctx context.Context, q core.TemplateQuery) ([]core.Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Template, 0)
	for _, t := range s.templates {
		if q.OwnerType != "" && t.OwnerType != q.OwnerType {
			continue
		}
		if q.OwnerID != "" && t.OwnerID != q.OwnerID {
			continue
		}
		if q.CommunityID != "" && t.CommunityID != q.CommunityID {
			continue
		}
		if q.OnlyPublic && !t.IsPublic {
			continue
		}
		if !contains(t.Name, q.Text) && !contains(t.Description, q.Text) {
			continue
		}
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (s *MemoryStore) CreateTemplateVersion(ctx context.Context, v core.TemplateVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(v.TemplateID, fmt.Sprint(v.Version))
	if _, exists := s.tmplVersions[k]; exists {
		return fmt.Errorf("template version %s v%d already exists (versions are immutable)", v.TemplateID, v.Version)
	}
	s.tmplVersions[k] = v
	return nil
}

func (s *MemoryStore) GetTemplateVersion(ctx context.Context, templateID string, version int) (core.TemplateVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.tmplVersions[key(templateID, fmt.Sprint(version))]
	if !ok {
		return core.TemplateVersion{}, fmt.Errorf("template version not found")
	}
	return v, nil
}

func (s *MemoryStore) ListTemplateVersions(ctx context.Context, templateID string) ([]core.TemplateVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.TemplateVersion, 0)
	for _, v := range s.tmplVersions {
		if v.TemplateID == templateID {
			list = append(list, v)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

// --- Loadouts ---

func (s *MemoryStore) CreateLoadout(ctx context.Context, l core.Loadout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.loadouts[l.ID]; exists {
		return fmt.Errorf("loadout %s already exists", l.ID)
	}
	s.loadouts[l.ID] = l
	return nil
}

func (s *MemoryStore) UpdateLoadout(ctx context.Context, l core.Loadout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.loadouts[l.ID]; !ok {
		return fmt.Errorf("loadout not found")
	}
	s.loadouts[l.ID] = l
	return nil
}

func (s *MemoryStore) DeleteLoadout(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.loadouts, id)
	delete(s.entries, id)
	return nil
}

func (s *MemoryStore) GetLoadout(ctx context.Context, id string) (core.Loadout, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.loadouts[id]
	if !ok {
		return core.Loadout{}, fmt.Errorf("loadout not found")
	}
	return l, nil
}

func (s *MemoryStore) ListLoadouts(ctx context.Context, q core.DiscoverQuery) ([]core.Loadout, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Loadout, 0)
	for _, l := range s.loadouts {
		if q.ProfileID != "" && l.OwnerProfileID != q.ProfileID {
			continue
		}
		if q.CommunityID != "" && l.CommunityID != q.CommunityID {
			continue
		}
		if q.TemplateID != "" && l.TemplateID != q.TemplateID {
			continue
		}
		if q.Status != "" && l.Status != q.Status {
			continue
		}
		if q.OnlyPublic && l.Visibility != core.VisibilityPublic {
			continue
		}
		if !contains(l.Name, q.Text) && !contains(l.Description, q.Text) {
			continue
		}
		list = append(list, l)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
	if q.Limit > 0 && len(list) > q.Limit {
		list = list[:q.Limit]
	}
	return list, nil
}

func (s *MemoryStore) ReplaceLoadoutEntries(ctx context.Context, loadoutID string, entries []core.LoadoutEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]core.LoadoutEntry, len(entries))
	copy(copied, entries)
	s.entries[loadoutID] = copied
	return nil
}

func (s *MemoryStore) ListLoadoutEntries(ctx context.Context, loadoutID string) ([]core.LoadoutEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := s.entries[loadoutID]
	out := make([]core.LoadoutEntry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}
