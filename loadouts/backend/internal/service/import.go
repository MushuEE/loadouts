package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
	"github.com/gmccloskey/loadouts/backend/internal/importer"
)

// ImportService turns retailer URLs into catalog items.
//
// The flow is deliberately two-step. Scraped product data is never fully trustworthy —
// weights are missing, categories are retailer-specific, names carry marketing noise — so
// Preview returns an editable draft and writes nothing, and Commit persists exactly what
// the user confirmed. That keeps bad scrapes out of the shared catalog while still saving
// the user most of the typing.
type ImportService struct {
	store     db.Store
	inventory *InventoryService
	fetcher   importer.Fetcher
}

func NewImportService(store db.Store, inventory *InventoryService, fetcher importer.Fetcher) *ImportService {
	if fetcher == nil {
		fetcher = importer.NewHTTPFetcher()
	}
	return &ImportService{store: store, inventory: inventory, fetcher: fetcher}
}

// Preview statuses tell the client which UI to show.
const (
	// StatusParsed: we read the page and prefilled the form.
	StatusParsed = "parsed"
	// StatusManual: we identified the product but could not read the page. The user fills
	// in the details; the link and dedupe key are still correct.
	StatusManual = "manual"
	// StatusExisting: this product is already in the catalog. Offer it instead of a
	// duplicate.
	StatusExisting = "existing"
)

// ImportPreview is the result of inspecting a URL. It never mutates state.
type ImportPreview struct {
	Status string         `json:"status"`
	Draft  importer.Draft `json:"draft"`
	// SupplierName is the display name for the resolved retailer.
	SupplierName string `json:"supplier_name"`
	// AffiliateURL is where the "Buy" button will point once committed.
	AffiliateURL string `json:"affiliate_url"`
	// SuggestedID is the catalog ID the item would get, exposed so the preview can show it.
	SuggestedID string `json:"suggested_id"`
	// ExistingItem is populated only when Status is StatusExisting.
	ExistingItem *core.Item `json:"existing_item,omitempty"`
	// Warning explains a degraded result (blocked scrape, missing weight) in plain
	// language. It is not an error: the import can still proceed.
	Warning string `json:"warning,omitempty"`
}

// Preview inspects a product URL and returns an editable draft.
func (s *ImportService) Preview(ctx context.Context, rawURL string) (ImportPreview, error) {
	target, err := importer.Canonicalize(rawURL)
	if err != nil {
		return ImportPreview{}, translateImportError(err)
	}

	supplier := s.supplierFor(ctx, target)
	preview := ImportPreview{
		Draft:        importer.Draft{Target: target},
		SupplierName: supplier.Name,
		AffiliateURL: affiliateURL(supplier, target),
	}

	// Dedupe before fetching: no point scraping a page whose product we already have,
	// and it keeps a popular item from being imported ten times.
	if existingID, err := s.store.GetItemBySource(ctx, target.SupplierID, target.ProductID); err == nil && existingID != "" {
		if item, err := s.store.GetItem(ctx, existingID); err == nil {
			preview.Status = StatusExisting
			preview.ExistingItem = &item
			preview.SuggestedID = item.ID
			preview.Draft.Name = item.Name
			preview.Draft.Category = item.Category
			preview.Draft.ImageURL = item.ImageURL
			return preview, nil
		}
	}

	body, fetchErr := s.fetcher.Fetch(ctx, target.CanonicalURL)
	if fetchErr != nil {
		// A blocked internal address is a hard failure, not a degraded one. Unlike a
		// retailer bot wall, there is no legitimate import behind a private IP, and
		// falling back to manual entry would leave a catalog item pointing at
		// infrastructure.
		if errors.Is(fetchErr, importer.ErrBlockedHost) {
			return ImportPreview{}, translateImportError(fetchErr)
		}
		// A blocked or unreachable store is a degraded success. We still know the
		// supplier, the product ID, and the affiliate URL, so the user can fill in the
		// rest and end up with a correctly linked, correctly deduped item.
		preview.Status = StatusManual
		preview.Warning = fmt.Sprintf("Couldn't read the product page (%s). Fill in the details below and the link will still work.", fetchErr)
		preview.SuggestedID = s.suggestID(ctx, target)
		return preview, nil
	}

	preview.Draft = importer.Extract(body, target)
	if !preview.Draft.IsUsable() {
		preview.Status = StatusManual
		preview.Warning = "We reached the page but couldn't recognize a product on it. Please fill in the details below."
	} else {
		preview.Status = StatusParsed
		preview.Warning = missingFieldWarning(preview.Draft)
	}
	preview.SuggestedID = s.suggestID(ctx, target, preview.Draft.Name)
	return preview, nil
}

// CommitRequest is the confirmed draft. Everything the user could edit in the preview is
// accepted here; nothing is re-scraped, so what they saw is what gets saved.
type CommitRequest struct {
	URL         string  `json:"url"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Brand       string  `json:"brand"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	WeightG     float64 `json:"weight_g"`
	CostCents   float64 `json:"cost_cents"`
	Currency    string  `json:"currency"`
	Consumable  bool    `json:"consumable"`
}

// ImportResult reports what Commit did. Created is false when the URL turned out to
// already be in the catalog, which makes the endpoint idempotent.
type ImportResult struct {
	Item    core.Item `json:"item"`
	Created bool      `json:"created"`
}

// Commit persists a confirmed draft as a global catalog item plus its retailer source.
//
// Imported items enter the global catalog immediately so everyone benefits, but flagged
// origin=import / verified=false so the provenance is never lost and moderation can be
// layered on later without a backfill.
func (s *ImportService) Commit(ctx context.Context, req CommitRequest, profileID string) (ImportResult, error) {
	if profileID == "" {
		return ImportResult{}, fmt.Errorf("%w: importing requires an acting profile", core.ErrForbidden)
	}
	target, err := importer.Canonicalize(req.URL)
	if err != nil {
		return ImportResult{}, translateImportError(err)
	}
	if strings.TrimSpace(req.Name) == "" {
		return ImportResult{}, fmt.Errorf("%w: name is required", core.ErrInvalid)
	}
	if req.WeightG < 0 || req.CostCents < 0 {
		return ImportResult{}, fmt.Errorf("%w: weight and cost cannot be negative", core.ErrInvalid)
	}

	// Idempotency: a double-clicked confirm button must not create two items.
	if existingID, err := s.store.GetItemBySource(ctx, target.SupplierID, target.ProductID); err == nil && existingID != "" {
		item, err := s.store.GetItem(ctx, existingID)
		if err != nil {
			return ImportResult{}, err
		}
		return ImportResult{Item: item, Created: false}, nil
	}

	item := core.Item{
		ID:            s.suggestID(ctx, target, req.Name),
		Name:          strings.TrimSpace(req.Name),
		Category:      strings.TrimSpace(req.Category),
		ImageURL:      req.ImageURL,
		ProvidedSlots: core.SlotList{},
		Origin:        core.OriginImport,
		Verified:      false,
		ImportedBy:    profileID,
		BaseMetadata: core.Metadata{
			// The core namespace is what the loadout stats engine reads, so populating
			// it here is what makes an imported item immediately usable in a loadout.
			core.CoreNamespace: map[string]interface{}{
				core.KeyWeightG:     req.WeightG,
				core.KeyCostCents:   req.CostCents,
				core.KeyConsumable:  req.Consumable,
				core.KeyDescription: strings.TrimSpace(req.Description),
			},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Import provenance lives in its own namespace rather than polluting core.
	provenance := map[string]interface{}{
		"supplier":    target.SupplierID,
		"product_id":  target.ProductID,
		"source_url":  target.CanonicalURL,
		"imported_at": time.Now().UTC().Format(time.RFC3339),
	}
	if brand := strings.TrimSpace(req.Brand); brand != "" {
		provenance["brand"] = brand
	}
	item.BaseMetadata["import"] = provenance

	if err := s.inventory.CreateItem(ctx, item); err != nil {
		return ImportResult{}, err
	}

	// Make sure the supplier row exists before pointing a source at it. The registry in
	// internal/importer is the source of truth for which retailers we understand, so a
	// fresh database never has to be manually topped up to accept an import.
	supplier := s.supplierFor(ctx, target)
	if err := s.store.UpsertSupplier(ctx, supplier); err != nil {
		return ImportResult{}, fmt.Errorf("saving supplier: %w", err)
	}

	source := core.ItemSource{
		ID:          core.NewID("src"),
		ItemID:      item.ID,
		SupplierID:  target.SupplierID,
		ProductID:   target.ProductID,
		SourceURL:   target.CanonicalURL,
		Price:       req.CostCents / 100,
		Currency:    defaultString(req.Currency, "USD"),
		LastUpdated: time.Now().UTC(),
	}
	if err := s.store.UpsertItemSource(ctx, source); err != nil {
		return ImportResult{}, fmt.Errorf("saving item source: %w", err)
	}

	return ImportResult{Item: item, Created: true}, nil
}

// SupportedSuppliers lists the retailers the importer has URL rules for, so the UI can
// tell the user what to expect before they paste anything.
func (s *ImportService) SupportedSuppliers() []map[string]string {
	out := make([]map[string]string, 0)
	for _, sup := range importer.Suppliers() {
		out = append(out, map[string]string{
			"id":       sup.ID,
			"name":     sup.Name,
			"base_url": sup.BaseURL,
		})
	}
	return out
}

// supplierFor prefers the stored supplier row (it holds the live affiliate template) and
// falls back to the code registry when the row hasn't been created yet.
func (s *ImportService) supplierFor(ctx context.Context, target importer.Target) core.Supplier {
	if stored, err := s.store.GetSupplier(ctx, target.SupplierID); err == nil && stored.ID != "" {
		return stored
	}
	if reg, ok := importer.SupplierByID(target.SupplierID); ok {
		return core.Supplier{
			ID:                reg.ID,
			Name:              reg.Name,
			BaseURL:           reg.BaseURL,
			AffiliateTemplate: reg.AffiliateTemplate,
		}
	}
	return core.Supplier{
		ID:                target.SupplierID,
		Name:              target.Host,
		AffiliateTemplate: "{{.ProductID}}",
	}
}

// suggestID builds a readable, unique catalog ID. Human-readable IDs match the existing
// seeded catalog (tent-copper-spur-ul2) and read better in URLs than a random token.
func (s *ImportService) suggestID(ctx context.Context, target importer.Target, name ...string) string {
	base := ""
	if len(name) > 0 {
		base = core.Slugify(name[0])
	}
	if base == "" {
		base = core.Slugify(target.SupplierID + "-" + target.ProductID)
	}
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-")
	}
	if base == "" {
		return core.NewID("itm")
	}

	candidate := base
	for i := 2; i < 12; i++ {
		if _, err := s.store.GetItem(ctx, candidate); err != nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	// Pathological collision count; fall back to a guaranteed-unique ID.
	return core.NewID("itm")
}

func affiliateURL(supplier core.Supplier, target importer.Target) string {
	url, err := core.ResolveSourceURL(supplier, core.ItemSource{ProductID: target.ProductID})
	if err != nil || url == "" {
		return target.CanonicalURL
	}
	return url
}

// missingFieldWarning nudges the user toward the fields that matter most for a loadout.
// Weight is called out specifically because it is what the stats engine runs on.
func missingFieldWarning(d importer.Draft) string {
	var missing []string
	if !d.HasWeight {
		missing = append(missing, "weight")
	}
	if !d.HasPrice {
		missing = append(missing, "price")
	}
	if d.Category == "" {
		missing = append(missing, "category")
	}
	if len(missing) == 0 {
		return ""
	}
	return "We couldn't find the " + strings.Join(missing, ", ") + " on the page. Add it below so this item works in loadout stats."
}

// translateImportError maps importer sentinels onto the core sentinels the HTTP layer
// already knows how to turn into status codes.
func translateImportError(err error) error {
	switch {
	case errors.Is(err, importer.ErrInvalidURL), errors.Is(err, importer.ErrNotAProduct):
		return fmt.Errorf("%w: %s", core.ErrInvalid, err)
	case errors.Is(err, importer.ErrBlockedHost):
		return fmt.Errorf("%w: %s", core.ErrForbidden, err)
	}
	return err
}
