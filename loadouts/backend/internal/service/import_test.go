package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/importer"
)

const reiTentPage = `<html><head>
<title>Big Agnes Copper Spur HV UL2 Tent | REI Co-op</title>
<script type="application/ld+json">
{
  "@type": "Product",
  "name": "Big Agnes Copper Spur HV UL2 Tent",
  "image": "https://www.rei.com/media/894303.jpg",
  "brand": {"name": "Big Agnes"},
  "weight": {"value": "3", "unitCode": "LBR"},
  "offers": {"price": "549.95", "priceCurrency": "USD"}
}
</script></head><body></body></html>`

// newImportHarness builds an import service backed by canned pages, so the whole flow
// runs deterministically with no network access.
func newImportHarness(t *testing.T, pages map[string][]byte, fetchErr error) (*harness, *ImportService) {
	t.Helper()
	h := newHarness(t)
	fetcher := &importer.StaticFetcher{Pages: pages, Err: fetchErr}
	return h, NewImportService(h.store, h.inventory, fetcher)
}

func TestImportPreviewParsesProductPage(t *testing.T) {
	pages := map[string][]byte{"https://www.rei.com/product/894303": []byte(reiTentPage)}
	_, svc := newImportHarness(t, pages, nil)

	preview, err := svc.Preview(context.Background(),
		"https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2?color=grey")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.Status != StatusParsed {
		t.Errorf("status = %q, want %q (warning: %s)", preview.Status, StatusParsed, preview.Warning)
	}
	if preview.Draft.Name != "Big Agnes Copper Spur HV UL2 Tent" {
		t.Errorf("name = %q", preview.Draft.Name)
	}
	if preview.SupplierName != "REI" {
		t.Errorf("supplierName = %q", preview.SupplierName)
	}
	if !preview.Draft.HasWeight {
		t.Error("expected a weight to be extracted")
	}
	// The affiliate template must be applied so the preview shows the real outbound link.
	wantURL := "https://www.rei.com/product/894303?cm_mmc=aff_AL-_-loadouts"
	if preview.AffiliateURL != wantURL {
		t.Errorf("affiliateURL = %q, want %q", preview.AffiliateURL, wantURL)
	}
	if preview.SuggestedID != "big-agnes-copper-spur-hv-ul2-tent" {
		t.Errorf("suggestedID = %q", preview.SuggestedID)
	}
}

// A blocked retailer (the expected Amazon case) must still yield a usable preview: we
// know the supplier and ASIN, so the user can fill in the rest by hand.
func TestImportPreviewFallsBackToManualWhenBlocked(t *testing.T) {
	_, svc := newImportHarness(t, nil, fmt.Errorf("the store returned HTTP 503"))

	preview, err := svc.Preview(context.Background(), "https://www.amazon.com/dp/B07P8ZQZ8Z")
	if err != nil {
		t.Fatalf("a blocked fetch must not be a hard error, got: %v", err)
	}
	if preview.Status != StatusManual {
		t.Errorf("status = %q, want %q", preview.Status, StatusManual)
	}
	if preview.Warning == "" {
		t.Error("expected a warning explaining the degraded result")
	}
	if preview.Draft.Target.ProductID != "B07P8ZQZ8Z" {
		t.Errorf("productID = %q, want the ASIN", preview.Draft.Target.ProductID)
	}
	if preview.AffiliateURL != "https://www.amazon.com/dp/B07P8ZQZ8Z?tag=loadouts-20" {
		t.Errorf("affiliateURL = %q", preview.AffiliateURL)
	}
}

func TestImportCommitCreatesItemWithProvenanceAndStats(t *testing.T) {
	pages := map[string][]byte{"https://www.rei.com/product/894303": []byte(reiTentPage)}
	h, svc := newImportHarness(t, pages, nil)
	profile := h.profile(t, "gearhead")
	ctx := context.Background()

	result, err := svc.Commit(ctx, CommitRequest{
		URL:       "https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2",
		Name:      "Big Agnes Copper Spur HV UL2",
		Category:  "shelter",
		Brand:     "Big Agnes",
		WeightG:   1360,
		CostCents: 54995,
	}, profile.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !result.Created {
		t.Error("expected Created = true for a new item")
	}

	// Provenance: imported items are usable but distinguishable from curated ones.
	if result.Item.Origin != core.OriginImport {
		t.Errorf("origin = %q, want %q", result.Item.Origin, core.OriginImport)
	}
	if result.Item.Verified {
		t.Error("a freshly imported item must not be marked verified")
	}
	if result.Item.ImportedBy != profile.ID {
		t.Errorf("importedBy = %q, want %q", result.Item.ImportedBy, profile.ID)
	}

	// The core namespace is what the stats engine reads; without it the import is inert.
	coreNS, ok := result.Item.BaseMetadata[core.CoreNamespace].(map[string]interface{})
	if !ok {
		t.Fatalf("core namespace missing from %#v", result.Item.BaseMetadata)
	}
	if coreNS[core.KeyWeightG] != 1360.0 {
		t.Errorf("core.weight_g = %v, want 1360", coreNS[core.KeyWeightG])
	}
	if coreNS[core.KeyCostCents] != 54995.0 {
		t.Errorf("core.cost_cents = %v, want 54995", coreNS[core.KeyCostCents])
	}

	importNS, ok := result.Item.BaseMetadata["import"].(map[string]interface{})
	if !ok {
		t.Fatal("import namespace missing")
	}
	if importNS["supplier"] != "rei" || importNS["product_id"] != "894303" {
		t.Errorf("import provenance = %#v", importNS)
	}

	// The source row is what powers dedupe and the affiliate link.
	sources, err := h.store.GetItemSources(ctx, result.Item.ID)
	if err != nil || len(sources) != 1 {
		t.Fatalf("GetItemSources = %v, %v; want exactly one source", sources, err)
	}
	if sources[0].SupplierID != "rei" || sources[0].ProductID != "894303" {
		t.Errorf("source = %#v", sources[0])
	}
	if sources[0].Price != 549.95 {
		t.Errorf("source price = %v, want 549.95", sources[0].Price)
	}
}

// Importing the same product twice, from differently-decorated URLs, must not create a
// second catalog entry. This is the whole reason item_sources has a uniqueness key.
func TestImportDedupesAcrossURLVariants(t *testing.T) {
	pages := map[string][]byte{"https://www.rei.com/product/894303": []byte(reiTentPage)}
	h, svc := newImportHarness(t, pages, nil)
	profile := h.profile(t, "gearhead")
	ctx := context.Background()

	first, err := svc.Commit(ctx, CommitRequest{
		URL: "https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2", Name: "Copper Spur UL2", Category: "shelter",
	}, profile.ID)
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}

	second, err := svc.Commit(ctx, CommitRequest{
		URL: "https://rei.com/product/894303/totally-different-slug?utm_source=reddit", Name: "Copper Spur UL2 Again", Category: "shelter",
	}, profile.ID)
	if err != nil {
		t.Fatalf("second commit: %v", err)
	}

	if second.Created {
		t.Error("re-importing the same product must not create a second item")
	}
	if second.Item.ID != first.Item.ID {
		t.Errorf("second import resolved to %q, want the existing %q", second.Item.ID, first.Item.ID)
	}

	items, _ := h.store.ListItems(ctx, "")
	if len(items) != 1 {
		t.Errorf("catalog has %d items, want 1", len(items))
	}

	// And a preview of the same URL should now report it as already known.
	preview, err := svc.Preview(ctx, "https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Status != StatusExisting {
		t.Errorf("status = %q, want %q", preview.Status, StatusExisting)
	}
	if preview.ExistingItem == nil || preview.ExistingItem.ID != first.Item.ID {
		t.Errorf("existingItem = %#v", preview.ExistingItem)
	}
}

func TestImportCommitRequiresProfileAndName(t *testing.T) {
	h, svc := newImportHarness(t, nil, nil)
	profile := h.profile(t, "gearhead")
	ctx := context.Background()
	url := "https://www.rei.com/product/894303/tent"

	if _, err := svc.Commit(ctx, CommitRequest{URL: url, Name: "Tent"}, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("anonymous commit error = %v, want ErrForbidden", err)
	}
	if _, err := svc.Commit(ctx, CommitRequest{URL: url, Name: "   "}, profile.ID); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("blank name error = %v, want ErrInvalid", err)
	}
	if _, err := svc.Commit(ctx, CommitRequest{URL: url, Name: "Tent", WeightG: -5}, profile.ID); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("negative weight error = %v, want ErrInvalid", err)
	}
	if _, err := svc.Commit(ctx, CommitRequest{URL: "https://www.rei.com/search?q=tent", Name: "Tent"}, profile.ID); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("non-product URL error = %v, want ErrInvalid", err)
	}
}

// Two different products whose names slugify identically must not collide onto one ID.
func TestImportGeneratesUniqueIDsOnNameCollision(t *testing.T) {
	h, svc := newImportHarness(t, nil, fmt.Errorf("blocked"))
	profile := h.profile(t, "gearhead")
	ctx := context.Background()

	first, err := svc.Commit(ctx, CommitRequest{
		URL: "https://www.rei.com/product/1/thing", Name: "Trail Runner", Category: "shoes",
	}, profile.ID)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.Commit(ctx, CommitRequest{
		URL: "https://www.rei.com/product/2/thing", Name: "Trail Runner", Category: "shoes",
	}, profile.ID)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first.Item.ID == second.Item.ID {
		t.Fatalf("both imports got the same ID %q", first.Item.ID)
	}
	if first.Item.ID != "trail-runner" || second.Item.ID != "trail-runner-2" {
		t.Errorf("IDs = %q, %q; want trail-runner and trail-runner-2", first.Item.ID, second.Item.ID)
	}
}

// An unknown store still imports; it just resolves to the generic supplier and links
// straight through to the original URL.
func TestImportFromUnknownRetailer(t *testing.T) {
	page := []byte(`<html><head><meta property="og:title" content="Cottage Widget">
	<meta property="product:price:amount" content="42.00"></head></html>`)
	pages := map[string][]byte{"https://shop.example.com/gear/widget": page}
	h, svc := newImportHarness(t, pages, nil)
	profile := h.profile(t, "gearhead")
	ctx := context.Background()

	preview, err := svc.Preview(ctx, "https://shop.example.com/gear/widget?utm_source=x")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Draft.Name != "Cottage Widget" {
		t.Errorf("name = %q", preview.Draft.Name)
	}
	// The generic affiliate template is a passthrough, so the link is the product URL.
	if preview.AffiliateURL != "https://shop.example.com/gear/widget" {
		t.Errorf("affiliateURL = %q", preview.AffiliateURL)
	}

	result, err := svc.Commit(ctx, CommitRequest{
		URL: "https://shop.example.com/gear/widget", Name: "Cottage Widget", Category: "pack", CostCents: 4200,
	}, profile.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	sources, _ := h.store.GetItemSources(ctx, result.Item.ID)
	if len(sources) != 1 || sources[0].SupplierID != importer.GenericSupplierID {
		t.Errorf("sources = %#v, want one generic source", sources)
	}
}

// An imported item must drop straight into a loadout and move the stats, otherwise the
// import saved the user nothing.
func TestImportedItemWorksInLoadoutStats(t *testing.T) {
	pages := map[string][]byte{"https://www.rei.com/product/894303": []byte(reiTentPage)}
	h, svc := newImportHarness(t, pages, nil)
	profile := h.profile(t, "gearhead")
	ctx := context.Background()

	result, err := svc.Commit(ctx, CommitRequest{
		URL: "https://www.rei.com/product/894303/tent", Name: "Copper Spur UL2",
		Category: "shelter", WeightG: 1360, CostCents: 54995,
	}, profile.ID)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	loadout, err := h.loadouts.Create(ctx, profile.ID, CreateLoadoutRequest{
		Name: "Imported Gear Test", TemplateID: core.FreeformTemplateID,
	})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}

	detail, err := h.loadouts.ReplaceEntries(ctx, profile.ID, loadout.Loadout.ID, []core.LoadoutEntry{
		{ItemID: result.Item.ID, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}

	if detail.Stats.TotalWeightG != 1360 {
		t.Errorf("loadout totalWeightG = %v, want 1360", detail.Stats.TotalWeightG)
	}
	if detail.Stats.TotalCostCents != 54995 {
		t.Errorf("loadout totalCostCents = %v, want 54995", detail.Stats.TotalCostCents)
	}
}
