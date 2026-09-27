package seed

import (
	"context"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
	"github.com/gmccloskey/loadouts/backend/internal/service"
)

// The seed paperdolls are painted by hand in code, so run the whole seed against a memory
// store: every layout goes through the same validation the editor's saves do.
func TestSeedPublishesPaperdolls(t *testing.T) {
	ctx := context.Background()
	store := db.NewMemoryStore()
	inventory := service.NewInventoryService(store)
	community := service.NewCommunityService(store)
	templates := service.NewTemplateService(store, community)
	loadouts := service.NewLoadoutService(store, inventory, templates, community)
	if err := Run(ctx, Services{
		Store: store, Identity: service.NewIdentityService(store), Community: community,
		Templates: templates, Loadouts: loadouts, Inventory: inventory,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	list, err := templates.List(ctx, core.TemplateQuery{})
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	laidOut := map[string]bool{}
	for _, d := range list {
		laidOut[d.Template.Name] = d.Version.Paperdoll != nil
	}
	for name, want := range map[string]bool{
		"Basic Backpacking": true, "Trail Running": true, "Trail Meal Plan": true, "Daily Fit": false,
	} {
		if laidOut[name] != want {
			t.Errorf("%s: has paperdoll = %v, want %v", name, laidOut[name], want)
		}
	}

	// The PCT loadout pins the laid-out version.
	gearhead, err := store.GetProfileByHandle(ctx, "gearhead")
	if err != nil || !gearhead.IsSiteAdmin {
		t.Fatalf("gearhead should be the seeded site admin: %+v, %v", gearhead, err)
	}
	summaries, err := loadouts.Discover(ctx, core.DiscoverQuery{}, gearhead.ID)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	pinned := 0
	for _, s := range summaries {
		d, err := loadouts.Detail(ctx, s.Loadout.ID, gearhead.ID)
		if err != nil {
			t.Fatalf("detail: %v", err)
		}
		if d.Template.Template.Name != "Basic Backpacking" {
			continue
		}
		pinned++
		if d.Template.Version.Paperdoll == nil {
			t.Errorf("%s pins a Basic Backpacking version without a paperdoll", s.Loadout.Name)
		}
	}
	if pinned == 0 {
		t.Error("no seeded loadout uses Basic Backpacking")
	}
}
