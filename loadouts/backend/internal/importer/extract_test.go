package importer

import (
	"math"
	"strings"
	"testing"
)

// reiStylePage mirrors how REI and most modern retailers publish a product: a
// schema.org/Product JSON-LD block wrapped in an @graph, plus OpenGraph tags.
const reiStylePage = `<!DOCTYPE html>
<html><head>
<title>Big Agnes Copper Spur HV UL2 Tent | REI Co-op</title>
<meta property="og:title" content="Big Agnes Copper Spur HV UL2 Tent">
<meta property="og:image" content="https://www.rei.com/media/894303.jpg">
<meta property="og:description" content="A freestanding two-person backpacking tent.">
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {"@type": "BreadcrumbList", "itemListElement": []},
    {
      "@type": "Product",
      "name": "Big Agnes Copper Spur HV UL2 Tent",
      "description": "A freestanding two-person backpacking tent with steep walls.",
      "image": ["https://www.rei.com/media/894303-primary.jpg", "https://www.rei.com/media/894303-alt.jpg"],
      "brand": {"@type": "Brand", "name": "Big Agnes"},
      "sku": "894303",
      "category": "Camping &amp; Hiking/Tents",
      "weight": {"@type": "QuantitativeValue", "value": "3", "unitCode": "LBR"},
      "offers": {"@type": "Offer", "price": "549.95", "priceCurrency": "USD"}
    }
  ]
}
</script>
</head><body></body></html>`

// shopifyStylePage has a bare Product object (no @graph), a numeric price, an array of
// offers, and an ImageObject rather than a string.
const shopifyStylePage = `<html><head>
<title>Ultralight Dyneema Stuff Sack – Garage Grown Gear</title>
<script type="application/ld+json">
{
  "@type": "Product",
  "name": "Ultralight Dyneema Stuff Sack",
  "image": {"@type": "ImageObject", "url": "https://cdn.shopify.com/sack.jpg"},
  "brand": "Cottage Co",
  "offers": [{"@type": "Offer", "price": 34.5, "priceCurrency": "USD"}]
}
</script>
</head><body></body></html>`

// metaOnlyPage has no JSON-LD at all, which is the common case on older storefronts.
const metaOnlyPage = `<html><head>
<title>BRS 3000T Titanium Stove - 25g | TinyStoveCo</title>
<meta property="og:title" content="BRS 3000T Titanium Stove - 25g">
<meta property="og:image" content="https://tinystove.co/brs.png">
<meta property="product:price:amount" content="17.99">
<meta property="product:price:currency" content="usd">
<meta name="description" content="The classic ultralight canister stove.">
</head><body></body></html>`

// titleOnlyPage is the degenerate case: we should still return something editable.
const titleOnlyPage = `<html><head><title>Mystery Gadget</title></head><body></body></html>`

func TestExtractJSONLD(t *testing.T) {
	target := Target{SupplierID: "rei", ProductID: "894303"}
	d := Extract([]byte(reiStylePage), target)

	if d.Source != ViaJSONLD {
		t.Errorf("extracted via %q, want %q", d.Source, ViaJSONLD)
	}
	if d.Name != "Big Agnes Copper Spur HV UL2 Tent" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Brand != "Big Agnes" {
		t.Errorf("brand = %q, want %q", d.Brand, "Big Agnes")
	}
	// The first image of the array wins, not the OG tag, since JSON-LD runs first.
	if d.ImageURL != "https://www.rei.com/media/894303-primary.jpg" {
		t.Errorf("imageURL = %q", d.ImageURL)
	}
	if !d.HasPrice || d.CostCents != 54995 {
		t.Errorf("cost = %d (has=%v), want 54995", d.CostCents, d.HasPrice)
	}
	if d.Currency != "USD" {
		t.Errorf("currency = %q", d.Currency)
	}
	// 3 LBR must be converted to grams for the stats engine.
	if !d.HasWeight || math.Abs(d.WeightG-1360.78) > 0.05 {
		t.Errorf("weightG = %v (has=%v), want ~1360.78", d.WeightG, d.HasWeight)
	}
	if d.Category != "shelter" {
		t.Errorf("category = %q, want %q", d.Category, "shelter")
	}
	if d.Extras["sku"] != "894303" {
		t.Errorf("sku extra = %v", d.Extras["sku"])
	}
}

func TestExtractShopifyShapes(t *testing.T) {
	d := Extract([]byte(shopifyStylePage), Target{SupplierID: "garagegrowngear"})

	if d.Name != "Ultralight Dyneema Stuff Sack" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Brand != "Cottage Co" {
		t.Errorf("brand = %q", d.Brand)
	}
	if d.ImageURL != "https://cdn.shopify.com/sack.jpg" {
		t.Errorf("imageURL = %q", d.ImageURL)
	}
	// A JSON number, not a string, must still round to exact cents.
	if !d.HasPrice || d.CostCents != 3450 {
		t.Errorf("cost = %d, want 3450", d.CostCents)
	}
}

func TestExtractMetaTagsOnly(t *testing.T) {
	d := Extract([]byte(metaOnlyPage), Target{SupplierID: GenericSupplierID})

	if d.Source != ViaMeta {
		t.Errorf("extracted via %q, want %q", d.Source, ViaMeta)
	}
	if d.Name != "BRS 3000T Titanium Stove - 25g" {
		t.Errorf("name = %q", d.Name)
	}
	if !d.HasPrice || d.CostCents != 1799 {
		t.Errorf("cost = %d, want 1799", d.CostCents)
	}
	if d.Currency != "USD" {
		t.Errorf("currency = %q, want normalized USD", d.Currency)
	}
	// No weight field anywhere, so the "25g" in the product name is the fallback.
	if !d.HasWeight || d.WeightG != 25 {
		t.Errorf("weightG = %v, want 25 parsed from the name", d.WeightG)
	}
	if d.Category != "kitchen" {
		t.Errorf("category = %q, want kitchen", d.Category)
	}
}

func TestExtractTitleFallback(t *testing.T) {
	d := Extract([]byte(titleOnlyPage), Target{})

	if d.Source != ViaTitle {
		t.Errorf("extracted via %q, want %q", d.Source, ViaTitle)
	}
	if d.Name != "Mystery Gadget" {
		t.Errorf("name = %q", d.Name)
	}
	if !d.IsUsable() {
		t.Error("a draft with a name should be usable")
	}
	if d.HasWeight || d.HasPrice {
		t.Error("unknown weight/price must stay flagged as unknown, not default to zero")
	}
}

func TestExtractEmptyPage(t *testing.T) {
	d := Extract([]byte("<html></html>"), Target{SupplierID: "rei", ProductID: "1"})
	if d.IsUsable() {
		t.Error("an empty page should not produce a usable draft")
	}
	if d.Source != ViaNone {
		t.Errorf("source = %q, want %q", d.Source, ViaNone)
	}
	// The target survives even when extraction finds nothing, which is what lets the
	// user fall back to manual entry with a correct link.
	if d.Target.ProductID != "1" {
		t.Error("target must be preserved on an empty draft")
	}
}

func TestTrimSiteSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Big Agnes Copper Spur HV UL2 Tent | REI Co-op", "Big Agnes Copper Spur HV UL2 Tent"},
		{"Ultralight Dyneema Stuff Sack – Garage Grown Gear", "Ultralight Dyneema Stuff Sack"},
		// A dash inside a real product name must survive: the tail is too long to be a
		// store name, or contains digits.
		{"Durston X-Mid 1 - Solid Inner Tent Version 2", "Durston X-Mid 1 - Solid Inner Tent Version 2"},
		{"BRS 3000T Titanium Stove - 25g", "BRS 3000T Titanium Stove - 25g"},
		{"No Suffix Here", "No Suffix Here"},
	}
	for _, tc := range cases {
		if got := trimSiteSuffix(tc.in); got != tc.want {
			t.Errorf("trimSiteSuffix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractHandlesEscapedJSONLD(t *testing.T) {
	// Some CMSs HTML-escape the JSON-LD body; the extractor retries unescaped.
	page := `<html><head><script type="application/ld+json">
	{&quot;@type&quot;:&quot;Product&quot;,&quot;name&quot;:&quot;Escaped Widget&quot;}
	</script></head></html>`
	d := Extract([]byte(page), Target{})
	if d.Name != "Escaped Widget" {
		t.Errorf("name = %q, want %q", d.Name, "Escaped Widget")
	}
}

func TestExtractIgnoresNonProductJSONLD(t *testing.T) {
	page := `<html><head>
	<script type="application/ld+json">{"@type":"Organization","name":"REI Co-op"}</script>
	<title>Real Product Name</title>
	</head></html>`
	d := Extract([]byte(page), Target{})
	if strings.Contains(d.Name, "REI Co-op") {
		t.Errorf("name = %q, must not come from an Organization node", d.Name)
	}
	if d.Name != "Real Product Name" {
		t.Errorf("name = %q", d.Name)
	}
}
