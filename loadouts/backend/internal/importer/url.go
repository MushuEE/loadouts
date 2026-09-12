// Package importer turns a retailer product URL into a draft catalog item.
//
// The pipeline has four stages, each independently testable:
//
//	Canonicalize -> Fetch -> Extract -> Draft
//
// Canonicalize is pure string logic and never touches the network. This is deliberate:
// it means we can always identify the supplier and product ID (and therefore dedupe
// against the existing catalog and build a correct affiliate link) even when the
// retailer refuses to serve us the page. Amazon in particular blocks datacenter traffic
// aggressively, so "we couldn't read the page" has to be a degraded success, not a failure.
package importer

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Target identifies a product at a supplier, derived purely from its URL.
type Target struct {
	SupplierID string `json:"supplier_id"`
	// ProductID is the supplier's own identifier (an Amazon ASIN, an REI product number,
	// a Shopify handle). For unknown retailers it is the canonical URL itself.
	ProductID string `json:"product_id"`
	// CanonicalURL is the tracking-free URL for the product page.
	CanonicalURL string `json:"canonical_url"`
	Host         string `json:"host"`
}

// Supplier describes a retailer we know how to read URLs for. It mirrors core.Supplier
// but adds the host matching and product-ID extraction that only live in code.
type Supplier struct {
	ID                string
	Name              string
	BaseURL           string
	AffiliateTemplate string
	// Hosts are matched against the URL's hostname with the leading "www." stripped,
	// and also match any subdomain (so "amazon.com" matches "smile.amazon.com").
	Hosts []string
	// extract pulls the supplier's product ID out of a URL path. Returning "" means the
	// URL is a valid host but not a product page (a search page, the homepage, etc).
	extract func(u *url.URL) string
	// canonical rebuilds a clean product URL from the extracted ID. Optional; when nil
	// the URL is used as-is with its query string stripped.
	canonical func(s Supplier, productID string) string
}

// GenericSupplierID is used for retailers we have no specific rules for. The item still
// imports and still links correctly, it just doesn't earn affiliate revenue.
const GenericSupplierID = "other"

// asinPattern matches an Amazon ASIN: 10 characters, either the modern "B0..." form or
// a legacy ISBN-style numeric code.
var asinPattern = regexp.MustCompile(`\b(B[0-9A-Z]{9}|[0-9]{9}[0-9X])\b`)

// reiProductPattern matches the numeric product ID in /product/123456/some-slug.
var reiProductPattern = regexp.MustCompile(`/product/(\d+)`)

// patagoniaPattern matches the style code in /product/name/12345.html.
var patagoniaPattern = regexp.MustCompile(`/(\d{4,7})\.html`)

// suppliers is the registry of retailers we can read. Adding a store is a matter of
// appending an entry here; the extraction stage is generic across all of them.
var suppliers = []Supplier{
	{
		ID:                "amazon",
		Name:              "Amazon",
		BaseURL:           "https://www.amazon.com",
		AffiliateTemplate: "{{.BaseURL}}/dp/{{.ProductID}}?tag=loadouts-20",
		Hosts:             []string{"amazon.com", "amazon.co.uk", "amazon.ca", "amazon.de"},
		extract: func(u *url.URL) string {
			// Amazon URLs bury the ASIN in a dozen shapes (/dp/, /gp/product/,
			// /gp/aw/d/, /-/en/dp/). Matching the token anywhere in the path handles
			// all of them, including the ref= suffixes appended by the share button.
			if m := asinPattern.FindStringSubmatch(strings.ToUpper(u.Path)); m != nil {
				return m[1]
			}
			// Some share links carry the ASIN only in the query string.
			for _, k := range []string{"asin", "ASIN"} {
				if v := u.Query().Get(k); asinPattern.MatchString(strings.ToUpper(v)) {
					return strings.ToUpper(v)
				}
			}
			return ""
		},
		canonical: func(s Supplier, id string) string { return s.BaseURL + "/dp/" + id },
	},
	{
		ID:                "rei",
		Name:              "REI",
		BaseURL:           "https://www.rei.com",
		AffiliateTemplate: "{{.BaseURL}}/product/{{.ProductID}}?cm_mmc=aff_AL-_-loadouts",
		Hosts:             []string{"rei.com"},
		extract: func(u *url.URL) string {
			if m := reiProductPattern.FindStringSubmatch(u.Path); m != nil {
				return m[1]
			}
			return ""
		},
		canonical: func(s Supplier, id string) string { return s.BaseURL + "/product/" + id },
	},
	{
		ID:                "backcountry",
		Name:              "Backcountry",
		BaseURL:           "https://www.backcountry.com",
		AffiliateTemplate: "{{.BaseURL}}/{{.ProductID}}?aff=loadouts",
		Hosts:             []string{"backcountry.com"},
		extract: func(u *url.URL) string {
			// Backcountry product URLs are a single slug at the root: /brand-product-name.
			seg := lastPathSegment(u)
			if seg == "" || !strings.Contains(seg, "-") {
				return ""
			}
			return seg
		},
	},
	{
		ID:                "patagonia",
		Name:              "Patagonia",
		BaseURL:           "https://www.patagonia.com",
		AffiliateTemplate: "{{.BaseURL}}/product/{{.ProductID}}.html",
		Hosts:             []string{"patagonia.com"},
		extract: func(u *url.URL) string {
			if m := patagoniaPattern.FindStringSubmatch(u.Path); m != nil {
				return m[1]
			}
			return ""
		},
	},
	{
		ID:                "garagegrowngear",
		Name:              "Garage Grown Gear",
		BaseURL:           "https://www.garagegrowngear.com",
		AffiliateTemplate: "{{.BaseURL}}/products/{{.ProductID}}",
		Hosts:             []string{"garagegrowngear.com"},
		extract:           shopifyHandle,
		canonical:         func(s Supplier, id string) string { return s.BaseURL + "/products/" + id },
	},
}

// shopifyHandle extracts the product handle from the /products/<handle> path that every
// Shopify storefront uses. A large share of cottage gear makers run on Shopify.
func shopifyHandle(u *url.URL) string {
	parts := pathParts(u)
	for i, p := range parts {
		if p == "products" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func pathParts(u *url.URL) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Split(u.Path, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lastPathSegment(u *url.URL) string {
	parts := pathParts(u)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSuffix(parts[len(parts)-1], ".html")
}

// Suppliers returns the registry, for exposing the supported-retailer list to clients.
func Suppliers() []Supplier {
	out := make([]Supplier, len(suppliers))
	copy(out, suppliers)
	return out
}

// SupplierByID looks up a registry entry.
func SupplierByID(id string) (Supplier, bool) {
	for _, s := range suppliers {
		if s.ID == id {
			return s, true
		}
	}
	if id == GenericSupplierID {
		return genericSupplier(""), true
	}
	return Supplier{}, false
}

// genericSupplier synthesizes a passthrough supplier for an unknown host. The affiliate
// template is just the product ID, which for generic imports is the full canonical URL,
// so core.ResolveSourceURL returns a working link with no special-casing.
func genericSupplier(host string) Supplier {
	name := "Other"
	if host != "" {
		name = host
	}
	return Supplier{
		ID:                GenericSupplierID,
		Name:              name,
		BaseURL:           "",
		AffiliateTemplate: "{{.ProductID}}",
	}
}

// matchHost reports whether hostname belongs to the supplier, allowing subdomains.
func (s Supplier) matchHost(hostname string) bool {
	for _, h := range s.Hosts {
		if hostname == h || strings.HasSuffix(hostname, "."+h) {
			return true
		}
	}
	return false
}

// trackingParams are stripped when building a canonical URL so the same product shared
// from two different places dedupes to one catalog entry.
var trackingParams = map[string]bool{
	"ref": true, "ref_": true, "tag": true, "linkcode": true, "psc": true, "th": true,
	"utm_source": true, "utm_medium": true, "utm_campaign": true, "utm_term": true,
	"utm_content": true, "gclid": true, "fbclid": true, "cm_mmc": true, "srsltid": true,
	"ascsubtag": true, "smid": true, "pf_rd_r": true, "pf_rd_p": true, "sr": true, "qid": true,
}

// Canonicalize parses a product URL into a Target. It never performs network I/O.
//
// Unknown retailers are not an error: they resolve to the generic supplier with the
// cleaned URL as the product ID, so the item can still be imported and linked.
func Canonicalize(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, fmt.Errorf("%w: url is required", ErrInvalidURL)
	}
	// Tolerate a pasted "www.rei.com/..." with no scheme.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Target{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Target{}, fmt.Errorf("%w: only http and https URLs can be imported", ErrInvalidURL)
	}
	if u.Host == "" {
		return Target{}, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	hostname := strings.ToLower(u.Hostname())
	hostname = strings.TrimPrefix(hostname, "www.")

	// Reject obviously-internal targets here as well as at dial time. Canonicalize is the
	// only stage Commit runs, so without this a user could hand-create a catalog item
	// pointing at internal infrastructure without ever going through a fetch.
	if isInternalHostname(hostname) {
		return Target{}, fmt.Errorf("%w: %s is not a public address", ErrBlockedHost, hostname)
	}

	cleaned := *u
	cleaned.Host = u.Host
	cleaned.Fragment = ""
	cleaned.RawQuery = stripTracking(u.Query())

	for _, s := range suppliers {
		if !s.matchHost(hostname) {
			continue
		}
		productID := s.extract(u)
		if productID == "" {
			return Target{}, fmt.Errorf("%w: that looks like a %s page but not a product page", ErrNotAProduct, s.Name)
		}
		canonicalURL := cleaned.String()
		if s.canonical != nil {
			canonicalURL = s.canonical(s, productID)
		}
		return Target{
			SupplierID:   s.ID,
			ProductID:    productID,
			CanonicalURL: canonicalURL,
			Host:         hostname,
		}, nil
	}

	// Unknown retailer: still importable, just not monetized.
	canonicalURL := cleaned.String()
	return Target{
		SupplierID:   GenericSupplierID,
		ProductID:    canonicalURL,
		CanonicalURL: canonicalURL,
		Host:         hostname,
	}, nil
}

func stripTracking(q url.Values) string {
	for k := range q {
		if trackingParams[strings.ToLower(k)] {
			q.Del(k)
		}
	}
	return q.Encode()
}

// internalHostnames are names that never belong to a public storefront.
var internalHostnames = map[string]bool{
	"localhost": true, "localhost.localdomain": true,
	"metadata": true, "metadata.google.internal": true,
}

// internalSuffixes cover the conventional private-network TLDs.
var internalSuffixes = []string{".local", ".internal", ".localdomain", ".localhost"}

// isInternalHostname catches literal IPs in non-public ranges and well-known internal
// names. It does not resolve DNS: names that resolve to private IPs are caught later at
// dial time, which also covers DNS rebinding.
func isInternalHostname(hostname string) bool {
	if internalHostnames[hostname] {
		return true
	}
	for _, suffix := range internalSuffixes {
		if strings.HasSuffix(hostname, suffix) {
			return true
		}
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return !isPublicIP(ip)
	}
	return false
}
