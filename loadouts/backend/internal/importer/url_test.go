package importer

import (
	"errors"
	"math"
	"testing"
)

func TestCanonicalizeKnownRetailers(t *testing.T) {
	cases := []struct {
		name         string
		url          string
		wantSupplier string
		wantProduct  string
		wantCanon    string
	}{
		{
			name:         "rei product with slug and query",
			url:          "https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2?color=grey&size=2",
			wantSupplier: "rei",
			wantProduct:  "894303",
			wantCanon:    "https://www.rei.com/product/894303",
		},
		{
			name:         "rei outlet path",
			url:          "https://www.rei.com/rei-garage/product/221055/patagonia-nano-puff-jacket",
			wantSupplier: "rei",
			wantProduct:  "221055",
		},
		{
			name:         "amazon dp with ref suffix",
			url:          "https://www.amazon.com/Therm-Rest-NeoAir-UberLite/dp/B07P8ZQZ8Z/ref=sr_1_3?keywords=uberlite&qid=1700000000",
			wantSupplier: "amazon",
			wantProduct:  "B07P8ZQZ8Z",
			wantCanon:    "https://www.amazon.com/dp/B07P8ZQZ8Z",
		},
		{
			name:         "amazon gp product",
			url:          "https://www.amazon.com/gp/product/B01N5OBPBC",
			wantSupplier: "amazon",
			wantProduct:  "B01N5OBPBC",
		},
		{
			name:         "amazon mobile short form",
			url:          "https://www.amazon.com/gp/aw/d/B01N5OBPBC?psc=1",
			wantSupplier: "amazon",
			wantProduct:  "B01N5OBPBC",
		},
		{
			name:         "amazon regional domain",
			url:          "https://www.amazon.co.uk/dp/B07P8ZQZ8Z",
			wantSupplier: "amazon",
			wantProduct:  "B07P8ZQZ8Z",
		},
		{
			name:         "backcountry root slug",
			url:          "https://www.backcountry.com/black-diamond-alpine-carbon-cork-trekking-poles",
			wantSupplier: "backcountry",
			wantProduct:  "black-diamond-alpine-carbon-cork-trekking-poles",
		},
		{
			name:         "patagonia style code",
			url:          "https://www.patagonia.com/product/mens-capilene-cool-daily-hoody/45161.html",
			wantSupplier: "patagonia",
			wantProduct:  "45161",
		},
		{
			name:         "shopify handle",
			url:          "https://www.garagegrowngear.com/products/ultralight-dyneema-stuff-sack?variant=123",
			wantSupplier: "garagegrowngear",
			wantProduct:  "ultralight-dyneema-stuff-sack",
			wantCanon:    "https://www.garagegrowngear.com/products/ultralight-dyneema-stuff-sack",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Canonicalize(tc.url)
			if err != nil {
				t.Fatalf("Canonicalize(%q) returned error: %v", tc.url, err)
			}
			if got.SupplierID != tc.wantSupplier {
				t.Errorf("supplier = %q, want %q", got.SupplierID, tc.wantSupplier)
			}
			if got.ProductID != tc.wantProduct {
				t.Errorf("productID = %q, want %q", got.ProductID, tc.wantProduct)
			}
			if tc.wantCanon != "" && got.CanonicalURL != tc.wantCanon {
				t.Errorf("canonicalURL = %q, want %q", got.CanonicalURL, tc.wantCanon)
			}
		})
	}
}

// The same product shared from a phone, a search result, and an affiliate link must
// collapse to one catalog entry, otherwise dedupe via GetItemBySource is useless.
func TestCanonicalizeDedupesTrackingVariants(t *testing.T) {
	variants := []string{
		"https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2",
		"https://www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2?cm_mmc=aff_AL-_-someone",
		"https://rei.com/product/894303/different-slug-entirely?utm_source=reddit&utm_campaign=x",
		"www.rei.com/product/894303/big-agnes-copper-spur-hv-ul2",
	}

	var first Target
	for i, v := range variants {
		got, err := Canonicalize(v)
		if err != nil {
			t.Fatalf("variant %d: %v", i, err)
		}
		if i == 0 {
			first = got
			continue
		}
		if got.SupplierID != first.SupplierID || got.ProductID != first.ProductID {
			t.Errorf("variant %d resolved to (%s,%s), want (%s,%s)",
				i, got.SupplierID, got.ProductID, first.SupplierID, first.ProductID)
		}
	}
}

func TestCanonicalizeUnknownHostFallsBackToGeneric(t *testing.T) {
	got, err := Canonicalize("https://shop.example.com/gear/widget?utm_source=x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SupplierID != GenericSupplierID {
		t.Errorf("supplier = %q, want %q", got.SupplierID, GenericSupplierID)
	}
	// The generic product ID is the cleaned URL, so the affiliate template passes it
	// straight through as a working link.
	if got.ProductID != "https://shop.example.com/gear/widget" {
		t.Errorf("productID = %q, want the tracking-free URL", got.ProductID)
	}
}

func TestCanonicalizeRejectsNonProductAndBadInput(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want error
	}{
		{"empty", "", ErrInvalidURL},
		{"non-http scheme", "ftp://rei.com/product/1", ErrInvalidURL},
		{"javascript scheme", "javascript:alert(1)", ErrInvalidURL},
		{"known host search page", "https://www.rei.com/search?q=tent", ErrNotAProduct},
		{"known host homepage", "https://www.rei.com/", ErrNotAProduct},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(tc.url)
			if !errors.Is(err, tc.want) {
				t.Errorf("Canonicalize(%q) error = %v, want %v", tc.url, err, tc.want)
			}
		})
	}
}

func TestCanonicalizeRejectsInternalHosts(t *testing.T) {
	// Commit only runs Canonicalize, never Fetch, so this is the layer that stops a user
	// hand-creating a catalog item that points at internal infrastructure.
	internal := []string{
		"http://localhost:8080/product/1",
		"http://127.0.0.1/x",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/admin",
		"http://192.168.1.1/",
		"http://[::1]/x",
		"http://printer.local/x",
		"http://vault.internal/secret",
		"http://metadata.google.internal/",
	}
	for _, raw := range internal {
		if _, err := Canonicalize(raw); !errors.Is(err, ErrBlockedHost) {
			t.Errorf("Canonicalize(%q) error = %v, want ErrBlockedHost", raw, err)
		}
	}

	// A normal public store must still work.
	if _, err := Canonicalize("https://www.rei.com/product/894303/tent"); err != nil {
		t.Errorf("public URL unexpectedly rejected: %v", err)
	}
}

func TestParseWeightGrams(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"850 g", 850, true},
		{"850g", 850, true},
		{"1.2 kg", 1200, true},
		{"43.5 oz", 1233.2, true},
		{"2 lb", 907.18, true},
		{"2 lb 3 oz", 992.23, true}, // compound values must sum
		{"1,200 g", 1200, true},
		{"2 lbs. 3 ozs.", 992.23, true},
		{"Weight: 1 lb 4.2 oz", 572.68, true},
		{"", 0, false},
		{"one pound", 0, false},
		{"Copper Spur UL2", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseWeightGrams(tc.in)
		if ok != tc.ok {
			t.Errorf("ParseWeightGrams(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && math.Abs(got-tc.want) > 0.05 {
			t.Errorf("ParseWeightGrams(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParsePriceCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"$54.95", 5495, true},
		{"54.95", 5495, true},
		{"54.95 USD", 5495, true},
		{"1,299.00", 129900, true},
		{"375", 37500, true},
		{"", 0, false},
		{"call for price", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParsePriceCents(tc.in)
		if ok != tc.ok {
			t.Errorf("ParsePriceCents(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("ParsePriceCents(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestGuessCategory(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Big Agnes Copper Spur HV UL2 Tent", "shelter"},
		{"Hyperlite Mountain Gear 3400 Southwest Backpack", "pack"},
		{"Enlightened Equipment Enigma 20 Quilt", "sleep"},
		{"BRS 3000T Titanium Stove", "kitchen"},
		{"Altra Lone Peak 8 Trail Runner Shoe", "shoes"},
		{"Nitecore NB10000 Power Bank", "electronics"},
		{"Some Unclassifiable Gizmo", ""},
	}
	for _, tc := range cases {
		if got := GuessCategory(tc.in); got != tc.want {
			t.Errorf("GuessCategory(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
