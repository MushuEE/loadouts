// Package seed populates a demo dataset so the platform is explorable the moment it boots.
//
// It exercises every concept in the object model: two users with three profiles, two
// communities with a community metadata layer, a platform template plus a community
// template, a catalogue of real gear items, and published loadouts to discover and fork.
package seed

import (
	"context"
	"fmt"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
	"github.com/gmccloskey/loadouts/backend/internal/service"
)

// Services bundles everything the seeder needs.
type Services struct {
	Store     db.Store
	Identity  *service.IdentityService
	Community *service.CommunityService
	Templates *service.TemplateService
	Loadouts  *service.LoadoutService
	Inventory *service.InventoryService
}

// item is a compact description of a seed item; it expands into the core metadata namespace.
type item struct {
	ID         string
	Name       string
	Category   string
	WeightG    float64
	CostCents  float64
	Consumable bool
	Slots      core.SlotList
}

func (i item) toCore() core.Item {
	slots := i.Slots
	if slots == nil {
		slots = core.SlotList{}
	}
	return core.Item{
		ID:            i.ID,
		Name:          i.Name,
		Category:      i.Category,
		ProvidedSlots: slots,
		// The demo catalogue is hand-curated, so it is verified by definition. Only
		// retailer imports land unverified.
		Origin:   core.OriginCurated,
		Verified: true,
		BaseMetadata: core.Metadata{
			core.CoreNamespace: map[string]interface{}{
				core.KeyWeightG:    i.WeightG,
				core.KeyCostCents:  i.CostCents,
				core.KeyConsumable: i.Consumable,
			},
		},
	}
}

var catalogue = []item{
	{ID: "pack-hmg-3400", Name: "Hyperlite 3400 Southwest", Category: "pack", WeightG: 910, CostCents: 37500, Slots: core.SlotList{
		{ID: "main", Name: "Main Compartment", AcceptedCategories: []string{"universal"}, MaxItems: -1, Position: 0},
		{ID: "side-left", Name: "Left Side Pocket", AcceptedCategories: []string{"consumable", "shelter", "kitchen"}, MaxItems: 2, Position: 1},
		{ID: "side-right", Name: "Right Side Pocket", AcceptedCategories: []string{"consumable", "shelter", "kitchen"}, MaxItems: 2, Position: 2},
		{ID: "front-mesh", Name: "Front Mesh", AcceptedCategories: []string{"universal"}, MaxItems: -1, Position: 3},
	}},
	{ID: "tent-xmid-1", Name: "Durston X-Mid 1", Category: "shelter", WeightG: 795, CostCents: 24000},
	{ID: "tent-copper-spur-ul2", Name: "Big Agnes Copper Spur UL2", Category: "shelter", WeightG: 1360, CostCents: 54995},
	{ID: "quilt-ee-enigma-20", Name: "Enlightened Equipment Enigma 20°", Category: "sleep", WeightG: 550, CostCents: 35000},
	{ID: "pad-uberlite", Name: "Therm-a-Rest UberLite", Category: "sleep", WeightG: 250, CostCents: 18000},
	{ID: "shoes-lone-peak-8", Name: "Altra Lone Peak 8", Category: "shoes", WeightG: 600, CostCents: 14000},
	{ID: "shirt-capilene-cool", Name: "Patagonia Capilene Cool Daily Hoody", Category: "shirt", WeightG: 153, CostCents: 5500},
	{ID: "pants-strider-pro", Name: "Patagonia Strider Pro Shorts", Category: "pants", WeightG: 105, CostCents: 7900},
	{ID: "poles-alpine-cork", Name: "Black Diamond Alpine Carbon Cork", Category: "poles", WeightG: 480, CostCents: 19000},
	{ID: "stove-brs-3000t", Name: "BRS 3000T Stove", Category: "kitchen", WeightG: 25, CostCents: 1500},
	{ID: "pot-toaks-750", Name: "Toaks 750ml Titanium Pot", Category: "kitchen", WeightG: 100, CostCents: 3500, Slots: core.SlotList{
		{ID: "inside-pot", Name: "Inside Pot", AcceptedCategories: []string{"kitchen", "fuel", "consumable"}, MaxItems: -1, Position: 0},
	}},
	{ID: "fuel-isopro-100", Name: "IsoPro Fuel (100g)", Category: "fuel", WeightG: 200, CostCents: 600, Consumable: true},
	{ID: "food-day", Name: "Food Bag (1 Day)", Category: "consumable", WeightG: 700, CostCents: 1500, Consumable: true},
	{ID: "water-smartwater-1l", Name: "SmartWater 1L (Full)", Category: "consumable", WeightG: 1000, CostCents: 200, Consumable: true},
	{ID: "battery-nb10000", Name: "Nitecore NB10000", Category: "electronics", WeightG: 150, CostCents: 6000},
	{ID: "headlamp-nu25", Name: "Nitecore NU25", Category: "electronics", WeightG: 28, CostCents: 3500},
	{ID: "ditty-bag-dcf", Name: "DCF Ditty Bag", Category: "organizer", WeightG: 12, CostCents: 1500, Slots: core.SlotList{
		{ID: "ditty-1", Name: "Pocket 1", AcceptedCategories: []string{"universal"}, MaxItems: -1, Position: 0},
	}},
	// Streetwear, to prove the model is not backpacking-specific.
	{ID: "jacket-arcteryx-beta", Name: "Arc'teryx Beta LT Jacket", Category: "outerwear", WeightG: 430, CostCents: 45000},
	{ID: "sneaker-nb-990v6", Name: "New Balance 990v6", Category: "shoes", WeightG: 800, CostCents: 20000},
	{ID: "denim-full-count-1101", Name: "Full Count 1101 Denim", Category: "pants", WeightG: 700, CostCents: 32000},
}

// Run is idempotent: if the demo user already exists, seeding is skipped.
func Run(ctx context.Context, s Services) error {
	if err := s.Templates.EnsureFreeform(ctx); err != nil {
		return fmt.Errorf("ensure freeform template: %w", err)
	}
	if existing, err := s.Store.GetProfileByHandle(ctx, "gearhead"); err == nil && existing.ID != "" {
		return nil
	}

	// --- Items ---
	for _, it := range catalogue {
		if err := s.Inventory.CreateItem(ctx, it.toCore()); err != nil {
			return fmt.Errorf("seed item %s: %w", it.ID, err)
		}
	}

	// --- Users & profiles (one user, two profiles, to show the 1:many mapping) ---
	alex, err := s.Identity.CreateUser(ctx, "alex@example.com", "Alex Rivera")
	if err != nil {
		return err
	}
	gearhead, err := s.Identity.CreateProfile(ctx, core.Profile{
		UserID: alex.ID, Handle: "gearhead", DisplayName: "Alex | Gearhead",
		Bio: "Thru-hiker chasing a sub-5kg base weight.",
	})
	if err != nil {
		return err
	}
	fitcheck, err := s.Identity.CreateProfile(ctx, core.Profile{
		UserID: alex.ID, Handle: "fitcheck", DisplayName: "Alex | Fit Check",
		Bio: "Same human, different hobby.",
	})
	if err != nil {
		return err
	}

	jordan, err := s.Identity.CreateUser(ctx, "jordan@example.com", "Jordan Vale")
	if err != nil {
		return err
	}
	sponsor, err := s.Identity.CreateProfile(ctx, core.Profile{
		UserID: jordan.ID, Handle: "trailsponsor", DisplayName: "Trail Sponsor",
		Bio: "Sponsored athlete. Everything I actually carry.", IsSponsor: true,
	})
	if err != nil {
		return err
	}

	// --- Communities ---
	ul, err := s.Community.Create(ctx, gearhead.ID, core.Community{
		Slug: "ul-backpacking", Name: "UltraLight Backpacking",
		Description: "Grams are the enemy. Comfort is negotiable.",
		MetadataHint: core.Metadata{
			"ul_backpacking": map[string]interface{}{
				"ul_score": "number", "comfort": "number", "durability": "number",
			},
		},
	})
	if err != nil {
		return err
	}
	if _, err := s.Community.Join(ctx, ul.ID, sponsor.ID); err != nil {
		return err
	}

	nyc, err := s.Community.Create(ctx, fitcheck.ID, core.Community{
		Slug: "nyc-fashion", Name: "NYC Fashion",
		Description: "Fits, fabrics, and the walk test.",
		MetadataHint: core.Metadata{
			"nyc_fashion": map[string]interface{}{"drip_score": "number", "season": "string"},
		},
	})
	if err != nil {
		return err
	}

	// --- Community metadata layer: the exact example from the concept doc ---
	ulScores := map[string]map[string]interface{}{
		"tent-xmid-1":          {"ul_score": 9.1, "comfort": 7.4, "durability": 7.0},
		"tent-copper-spur-ul2": {"ul_score": 7.8, "comfort": 8.1, "durability": 4.2},
		"pack-hmg-3400":        {"ul_score": 8.6, "comfort": 6.9, "durability": 8.8},
		"quilt-ee-enigma-20":   {"ul_score": 9.4, "comfort": 8.0, "durability": 6.5},
		"pad-uberlite":         {"ul_score": 9.6, "comfort": 6.2, "durability": 3.1},
	}
	for itemID, scores := range ulScores {
		if _, err := s.Community.SetItemLayer(ctx, gearhead.ID, ul.ID, itemID, core.Metadata{
			"ul_backpacking": scores,
		}); err != nil {
			return fmt.Errorf("seed community layer %s: %w", itemID, err)
		}
	}

	// --- A profile layer: a public correction plus a private note ---
	if _, err := s.Inventory.SetProfileLayer(ctx, core.ProfileItemLayer{
		ProfileID: gearhead.ID,
		ItemID:    "tent-copper-spur-ul2",
		PublicMetadata: core.Metadata{
			// The global item never specified a material, so the user fills it in.
			"spec": map[string]interface{}{"material": "20D ripstop nylon"},
			core.CoreNamespace: map[string]interface{}{
				core.KeyWeightG: 1290.0, // Measured on my own scale, without the stuff sack.
			},
		},
		PrivateMetadata: core.Metadata{
			"notes": map[string]interface{}{
				"text": "Seam sealed 2024-04. Left pole segment is cracked, replace before the Sierra.",
			},
		},
	}); err != nil {
		return err
	}

	// --- Templates ---
	backpacking, err := s.Templates.Create(ctx, gearhead.ID, core.Template{
		Name: "Basic Backpacking", Description: "The classic big-four plus worn gear.",
		OwnerType: core.OwnerCommunity, CommunityID: ul.ID, IsPublic: true,
	}, core.SlotList{
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true, Position: 0},
		{ID: "sleep-bag", Name: "Sleeping Bag / Quilt", AcceptedCategories: []string{"sleep"}, Required: true, Position: 1},
		{ID: "sleep-pad", Name: "Sleeping Pad", AcceptedCategories: []string{"sleep"}, Required: true, Position: 2},
		{ID: "pack", Name: "Backpack", AcceptedCategories: []string{"pack"}, Required: true, Position: 3},
		{ID: "worn-torso", Name: "Worn: Torso", AcceptedCategories: []string{"shirt", "outerwear"}, Position: 4},
		{ID: "worn-legs", Name: "Worn: Legs", AcceptedCategories: []string{"pants"}, Position: 5},
		{ID: "worn-feet", Name: "Worn: Feet", AcceptedCategories: []string{"shoes"}, Position: 6},
		{ID: "poles", Name: "Trekking Poles", AcceptedCategories: []string{"poles"}, Position: 7},
	}, "Initial version.")
	if err != nil {
		return err
	}

	if _, err := s.Templates.Create(ctx, fitcheck.ID, core.Template{
		Name: "Daily Fit", Description: "Head-to-toe outfit template.",
		OwnerType: core.OwnerProfile, IsPublic: true,
	}, core.SlotList{
		{ID: "outer", Name: "Outerwear", AcceptedCategories: []string{"outerwear"}, Position: 0},
		{ID: "top", Name: "Top", AcceptedCategories: []string{"shirt"}, Position: 1},
		{ID: "bottom", Name: "Bottom", AcceptedCategories: []string{"pants"}, Required: true, Position: 2},
		{ID: "footwear", Name: "Footwear", AcceptedCategories: []string{"shoes"}, Required: true, Position: 3},
	}, "Initial version."); err != nil {
		return err
	}

	// --- Loadouts ---
	if err := seedLoadouts(ctx, s, gearhead, fitcheck, sponsor, ul, nyc, backpacking); err != nil {
		return err
	}
	return nil
}

func seedLoadouts(ctx context.Context, s Services, gearhead, fitcheck, sponsor core.Profile, ul, nyc core.Community, backpacking core.TemplateDetail) error {
	// A fully structured, nested loadout: pack -> side pocket -> pot -> fuel.
	packEntry := core.NewID("ent")
	potEntry := core.NewID("ent")

	pct, err := s.Loadouts.Create(ctx, gearhead.ID, service.CreateLoadoutRequest{
		Name:            "PCT Sierra Setup",
		Description:     "What I carried through the Sierra. Base weight under 5kg with the bear can off.",
		TemplateID:      backpacking.Template.ID,
		TemplateVersion: backpacking.Version.Version,
		CommunityID:     ul.ID,
		Entries: []core.LoadoutEntry{
			{ID: packEntry, SlotID: "pack", ItemID: "pack-hmg-3400", Position: 3},
			{SlotID: "shelter", ItemID: "tent-xmid-1", Position: 0},
			{SlotID: "sleep-bag", ItemID: "quilt-ee-enigma-20", Position: 1},
			{SlotID: "sleep-pad", ItemID: "pad-uberlite", Position: 2},
			{SlotID: "worn-torso", ItemID: "shirt-capilene-cool", Position: 4},
			{SlotID: "worn-legs", ItemID: "pants-strider-pro", Position: 5},
			{SlotID: "worn-feet", ItemID: "shoes-lone-peak-8", Position: 6},
			{SlotID: "poles", ItemID: "poles-alpine-cork", Position: 7},
			// Nested inside the pack:
			{ID: potEntry, SlotID: "side-left", ParentEntryID: packEntry, ItemID: "pot-toaks-750", Position: 0},
			{SlotID: "main", ParentEntryID: packEntry, ItemID: "food-day", Quantity: 5, Position: 1},
			{SlotID: "side-right", ParentEntryID: packEntry, ItemID: "water-smartwater-1l", Quantity: 2, Position: 2},
			// Nested inside the pot:
			{SlotID: "inside-pot", ParentEntryID: potEntry, ItemID: "stove-brs-3000t", Position: 0},
			{SlotID: "inside-pot", ParentEntryID: potEntry, ItemID: "fuel-isopro-100", Position: 1},
		},
	})
	if err != nil {
		return fmt.Errorf("seed PCT loadout: %w", err)
	}
	if _, err := s.Loadouts.Publish(ctx, gearhead.ID, pct.Loadout.ID, core.VisibilityPublic, ul.ID); err != nil {
		return fmt.Errorf("publish PCT loadout: %w", err)
	}

	// A sponsor loadout on the freeform template (no structure required).
	sponsored, err := s.Loadouts.Create(ctx, sponsor.ID, service.CreateLoadoutRequest{
		Name:        "My Everyday Trail Kit",
		Description: "Everything I actually use, sponsored or not.",
		TemplateID:  core.FreeformTemplateID,
		CommunityID: ul.ID,
		Entries: []core.LoadoutEntry{
			{SlotID: "free-0", ItemID: "tent-copper-spur-ul2", Position: 0},
			{SlotID: "free-1", ItemID: "battery-nb10000", Position: 1},
			{SlotID: "free-2", ItemID: "headlamp-nu25", Position: 2},
			{SlotID: "free-3", ItemID: "ditty-bag-dcf", Position: 3},
		},
	})
	if err != nil {
		return fmt.Errorf("seed sponsor loadout: %w", err)
	}
	if _, err := s.Loadouts.Publish(ctx, sponsor.ID, sponsored.Loadout.ID, core.VisibilityPublic, ul.ID); err != nil {
		return err
	}

	// A streetwear loadout, same primitives, different hobby.
	fit, err := s.Loadouts.Create(ctx, fitcheck.ID, service.CreateLoadoutRequest{
		Name:        "Rainy Tuesday Fit",
		Description: "Shell over denim, sneakers that survive puddles.",
		TemplateID:  core.FreeformTemplateID,
		CommunityID: nyc.ID,
		Entries: []core.LoadoutEntry{
			{SlotID: "free-0", ItemID: "jacket-arcteryx-beta", Position: 0},
			{SlotID: "free-1", ItemID: "denim-full-count-1101", Position: 1},
			{SlotID: "free-2", ItemID: "sneaker-nb-990v6", Position: 2},
		},
	})
	if err != nil {
		return fmt.Errorf("seed fit loadout: %w", err)
	}
	if _, err := s.Loadouts.Publish(ctx, fitcheck.ID, fit.Loadout.ID, core.VisibilityPublic, nyc.ID); err != nil {
		return err
	}

	return nil
}
