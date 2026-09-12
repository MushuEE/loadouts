package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// harness wires the full service graph over an in-memory store.
type harness struct {
	store     *db.MemoryStore
	identity  *IdentityService
	community *CommunityService
	templates *TemplateService
	loadouts  *LoadoutService
	inventory *InventoryService
	plugins   *PluginService
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := db.NewMemoryStore()
	inventory := NewInventoryService(store)
	community := NewCommunityService(store)
	templates := NewTemplateService(store, community)
	loadouts := NewLoadoutService(store, inventory, templates, community)

	if err := templates.EnsureFreeform(context.Background()); err != nil {
		t.Fatalf("ensure freeform: %v", err)
	}
	return &harness{
		store:     store,
		identity:  NewIdentityService(store),
		community: community,
		templates: templates,
		loadouts:  loadouts,
		inventory: inventory,
		plugins:   NewPluginService(store, community, loadouts, inventory, "https://sandbox.test"),
	}
}

func (h *harness) profile(t *testing.T, handle string) core.Profile {
	t.Helper()
	user, err := h.identity.CreateUser(context.Background(), handle+"@example.com", handle)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	profile, err := h.identity.CreateProfile(context.Background(), core.Profile{UserID: user.ID, Handle: handle})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return profile
}

func (h *harness) item(t *testing.T, id, category string, weight, cost float64, consumable bool, slots core.SlotList) {
	t.Helper()
	if slots == nil {
		slots = core.SlotList{}
	}
	err := h.inventory.CreateItem(context.Background(), core.Item{
		ID: id, Name: id, Category: category, ProvidedSlots: slots,
		BaseMetadata: core.Metadata{core.CoreNamespace: map[string]interface{}{
			core.KeyWeightG: weight, core.KeyCostCents: cost, core.KeyConsumable: consumable,
		}},
	})
	if err != nil {
		t.Fatalf("create item %s: %v", id, err)
	}
}

func TestIdentity_OneUserManyProfiles(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	user, err := h.identity.CreateUser(ctx, "alex@example.com", "Alex")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	for _, handle := range []string{"gearhead", "fitcheck"} {
		if _, err := h.identity.CreateProfile(ctx, core.Profile{UserID: user.ID, Handle: handle}); err != nil {
			t.Fatalf("create profile %s: %v", handle, err)
		}
	}

	profiles, err := h.identity.ListProfiles(ctx, user.ID)
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles for the user, got %d", len(profiles))
	}

	// Handles are globally unique.
	other, _ := h.identity.CreateUser(ctx, "jordan@example.com", "Jordan")
	if _, err := h.identity.CreateProfile(ctx, core.Profile{UserID: other.ID, Handle: "gearhead"}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("duplicate handle should conflict, got %v", err)
	}
}

func TestCommunity_MembershipAndAdminGating(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	member := h.profile(t, "member")
	h.item(t, "tent", "shelter", 1000, 20000, false, nil)

	community, err := h.community.Create(ctx, owner.ID, core.Community{Name: "UL Backpacking"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if community.Slug != "ul-backpacking" {
		t.Errorf("slug should be derived from the name, got %q", community.Slug)
	}
	if h.community.RoleOf(ctx, community.ID, owner.ID) != core.RoleOwner {
		t.Error("creator should be the owner")
	}

	if _, err := h.community.Join(ctx, community.ID, member.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	refreshed, _ := h.community.Get(ctx, community.ID)
	if refreshed.MemberCount != 2 {
		t.Errorf("member count = %d, want 2", refreshed.MemberCount)
	}

	// A plain member may not write the community layer.
	if _, err := h.community.SetItemLayer(ctx, member.ID, community.ID, "tent", core.Metadata{
		"ul_backpacking": map[string]interface{}{"ul_score": 9.0},
	}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("non-admin layer write should be forbidden, got %v", err)
	}
	if _, err := h.community.SetItemLayer(ctx, owner.ID, community.ID, "tent", core.Metadata{
		"ul_backpacking": map[string]interface{}{"ul_score": 9.0},
	}); err != nil {
		t.Errorf("admin layer write should succeed, got %v", err)
	}

	// The last owner cannot abandon the community.
	if err := h.community.Leave(ctx, community.ID, owner.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("last owner should not be able to leave, got %v", err)
	}
	if err := h.community.Leave(ctx, community.ID, member.ID); err != nil {
		t.Errorf("member should be able to leave, got %v", err)
	}
}

func TestTemplate_VersionsAreImmutable(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	stranger := h.profile(t, "stranger")

	created, err := h.templates.Create(ctx, author.ID, core.Template{Name: "Basic Backpacking"}, core.SlotList{
		{Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
	}, "v1")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if created.Version.Slots[0].ID != "shelter" {
		t.Errorf("slot id should be derived from the name, got %q", created.Version.Slots[0].ID)
	}

	if _, err := h.templates.PublishVersion(ctx, stranger.ID, created.Template.ID, nil, "hijack"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("only the owner may publish, got %v", err)
	}

	v2, err := h.templates.PublishVersion(ctx, author.ID, created.Template.ID, core.SlotList{
		{Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
		{Name: "Bear Canister", AcceptedCategories: []string{"organizer"}, Required: true},
	}, "add bear can")
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if v2.Template.LatestVersion != 2 {
		t.Errorf("latest version = %d, want 2", v2.Template.LatestVersion)
	}

	// v1 must be untouched.
	v1, err := h.templates.Detail(ctx, created.Template.ID, 1)
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	if len(v1.Version.Slots) != 1 {
		t.Errorf("v1 should still have 1 slot, got %d", len(v1.Version.Slots))
	}
}

func TestTemplate_DuplicateSlotIDsRejected(t *testing.T) {
	h := newHarness(t)
	author := h.profile(t, "author")

	_, err := h.templates.Create(context.Background(), author.ID, core.Template{Name: "Broken"}, core.SlotList{
		{ID: "shelter", Name: "Shelter"},
		{ID: "shelter", Name: "Shelter Again"},
	}, "")
	if !errors.Is(err, core.ErrInvalid) {
		t.Errorf("duplicate slot ids should be invalid, got %v", err)
	}
}
