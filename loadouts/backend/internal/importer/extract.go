package importer

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// Extraction reads a product page without an HTML parser dependency.
//
// We deliberately avoid pulling in golang.org/x/net/html: the two things we need
// (<script type="application/ld+json"> bodies and <meta> tags) are narrow, well-formed,
// machine-generated patterns that regex handles reliably. Regex on arbitrary HTML would
// be a mistake; regex on these two constructs is a reasonable trade for zero new deps.
//
// Strategies run most-trustworthy first and later strategies only fill gaps:
//
//  1. schema.org/Product JSON-LD  - structured, precise, emitted by most modern retailers
//  2. OpenGraph / meta tags       - near-universal, but coarse (no weight, often no price)
//  3. <title>                     - last resort so the user at least gets a name to edit

var (
	jsonLDPattern = regexp.MustCompile(`(?is)<script[^>]+type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
	metaPattern   = regexp.MustCompile(`(?is)<meta\s+([^>]+?)/?>`)
	attrPattern   = regexp.MustCompile(`(?is)([a-z:\-]+)\s*=\s*"([^"]*)"|([a-z:\-]+)\s*=\s*'([^']*)'`)
	titlePattern  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagStripper   = regexp.MustCompile(`(?s)<[^>]*>`)
)

// Extract builds a Draft from raw page HTML.
func Extract(body []byte, target Target) Draft {
	d := Draft{Target: target, Source: ViaNone}
	page := string(body)

	if applyJSONLD(page, &d) {
		d.Source = ViaJSONLD
	}

	metas := parseMetaTags(page)
	if fillFromMeta(metas, &d) && d.Source == ViaNone {
		d.Source = ViaMeta
	}

	if d.Name == "" {
		if m := titlePattern.FindStringSubmatch(page); m != nil {
			d.Name = cleanTitle(m[1])
			if d.Name != "" {
				d.Source = ViaTitle
			}
		}
	}

	// Retailers append their own name to the title ("Copper Spur UL2 | REI Co-op").
	d.Name = trimSiteSuffix(d.Name)

	if d.Category == "" {
		d.Category = GuessCategory(d.Name, d.Description, stringExtra(d, "breadcrumb"))
	}
	// A weight in the product name ("Fuel Canister 230g") is better than no weight.
	if !d.HasWeight {
		if g, ok := ParseWeightGrams(d.Name); ok {
			d.WeightG, d.HasWeight = g, true
		}
	}
	return d
}

// --- JSON-LD ---

func applyJSONLD(page string, d *Draft) bool {
	for _, m := range jsonLDPattern.FindAllStringSubmatch(page, -1) {
		var raw interface{}
		// JSON-LD blocks are HTML-escaped in some CMSs; try both forms.
		if err := json.Unmarshal([]byte(m[1]), &raw); err != nil {
			if err2 := json.Unmarshal([]byte(html.UnescapeString(m[1])), &raw); err2 != nil {
				continue
			}
		}
		if node := findProduct(raw); node != nil {
			applyProductNode(node, d)
			return true
		}
	}
	return false
}

// findProduct walks a JSON-LD document looking for the first schema.org Product. The
// document may be a single object, an array, or wrapped in an @graph, and the @type
// itself may be a string or an array.
func findProduct(node interface{}) map[string]interface{} {
	switch v := node.(type) {
	case map[string]interface{}:
		if isType(v["@type"], "Product") {
			return v
		}
		for _, key := range []string{"@graph", "mainEntity", "itemListElement"} {
			if sub, ok := v[key]; ok {
				if found := findProduct(sub); found != nil {
					return found
				}
			}
		}
	case []interface{}:
		for _, elem := range v {
			if found := findProduct(elem); found != nil {
				return found
			}
		}
	}
	return nil
}

func isType(v interface{}, want string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, want)
	case []interface{}:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.EqualFold(s, want) {
				return true
			}
		}
	}
	return false
}

func applyProductNode(p map[string]interface{}, d *Draft) {
	if name := str(p["name"]); name != "" {
		d.Name = cleanTitle(name)
	}
	if desc := str(p["description"]); desc != "" {
		d.Description = cleanTitle(desc)
	}
	if img := firstImage(p["image"]); img != "" {
		d.ImageURL = img
	}
	if brand := brandName(p["brand"]); brand != "" {
		d.Brand = brand
	}
	if cat := str(p["category"]); cat != "" {
		d.setExtra("retailer_category", cat)
		if guess := GuessCategory(cat); guess != "" {
			d.Category = guess
		}
	}
	for _, k := range []string{"sku", "mpn", "gtin13", "gtin12", "color", "material"} {
		d.setExtra(k, str(p[k]))
	}
	if w, ok := quantityGrams(p["weight"]); ok {
		d.WeightG, d.HasWeight = w, true
	}
	applyOffers(p["offers"], d)
}

// applyOffers reads price and currency. offers may be an object, an array of offers, or
// an AggregateOffer with lowPrice instead of price.
func applyOffers(node interface{}, d *Draft) {
	switch v := node.(type) {
	case []interface{}:
		for _, e := range v {
			applyOffers(e, d)
			if d.HasPrice {
				return
			}
		}
	case map[string]interface{}:
		for _, key := range []string{"price", "lowPrice", "highPrice"} {
			if cents, ok := ParsePriceCents(str(v[key])); ok {
				d.CostCents, d.HasPrice = cents, true
				break
			}
		}
		if cur := str(v["priceCurrency"]); cur != "" {
			d.Currency = strings.ToUpper(cur)
		}
		if !d.HasPrice {
			if spec, ok := v["priceSpecification"]; ok {
				applyOffers(spec, d)
			}
		}
	}
}

// quantityGrams reads a schema.org QuantitativeValue ({value, unitCode}) or a plain
// string like "2 lb 3 oz".
func quantityGrams(node interface{}) (float64, bool) {
	switch v := node.(type) {
	case string:
		return ParseWeightGrams(v)
	case map[string]interface{}:
		value := str(v["value"])
		if value == "" {
			return 0, false
		}
		// unitCode is a UN/CEFACT code: GRM grams, KGM kilograms, ONZ ounces, LBR pounds.
		unit := map[string]string{"GRM": "g", "KGM": "kg", "ONZ": "oz", "LBR": "lb"}[strings.ToUpper(str(v["unitCode"]))]
		if unit == "" {
			unit = str(v["unitText"])
		}
		if unit == "" {
			// A bare number in a weight field is conventionally grams.
			unit = "g"
		}
		return ParseWeightGrams(value + " " + unit)
	}
	return 0, false
}

// --- Meta tags ---

func parseMetaTags(page string) map[string]string {
	out := map[string]string{}
	for _, m := range metaPattern.FindAllStringSubmatch(page, -1) {
		attrs := map[string]string{}
		for _, a := range attrPattern.FindAllStringSubmatch(m[1], -1) {
			if a[1] != "" {
				attrs[strings.ToLower(a[1])] = a[2]
			} else {
				attrs[strings.ToLower(a[3])] = a[4]
			}
		}
		content, ok := attrs["content"]
		if !ok || content == "" {
			continue
		}
		for _, k := range []string{"property", "name", "itemprop"} {
			if key := strings.ToLower(attrs[k]); key != "" {
				if _, exists := out[key]; !exists {
					out[key] = html.UnescapeString(content)
				}
			}
		}
	}
	return out
}

func fillFromMeta(m map[string]string, d *Draft) bool {
	used := false
	pick := func(dst *string, keys ...string) {
		if *dst != "" {
			return
		}
		for _, k := range keys {
			if v := strings.TrimSpace(m[k]); v != "" {
				*dst = cleanTitle(v)
				used = true
				return
			}
		}
	}
	pick(&d.Name, "og:title", "twitter:title", "product:title")
	pick(&d.Description, "og:description", "twitter:description", "description")
	pick(&d.ImageURL, "og:image", "og:image:secure_url", "twitter:image", "image")
	pick(&d.Brand, "product:brand", "og:brand", "brand")

	if !d.HasPrice {
		for _, k := range []string{"product:price:amount", "og:price:amount", "twitter:data1", "price"} {
			if cents, ok := ParsePriceCents(m[k]); ok && cents > 0 {
				d.CostCents, d.HasPrice, used = cents, true, true
				break
			}
		}
	}
	if d.Currency == "" {
		for _, k := range []string{"product:price:currency", "og:price:currency"} {
			if v := strings.TrimSpace(m[k]); v != "" {
				d.Currency = strings.ToUpper(v)
				break
			}
		}
	}
	if !d.HasWeight {
		for _, k := range []string{"product:weight:value", "weight"} {
			if g, ok := ParseWeightGrams(m[k]); ok {
				d.WeightG, d.HasWeight, used = g, true, true
				break
			}
		}
	}
	return used
}

// --- helpers ---

func str(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		// JSON numbers arrive as float64; render prices like 54.95 without an exponent
		// and integers without a trailing ".00".
		if t == float64(int64(t)) {
			return strings.TrimSpace(strings.TrimSuffix(formatFloat(t), ".0"))
		}
		return formatFloat(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return ""
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func firstImage(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		for _, e := range t {
			if s := firstImage(e); s != "" {
				return s
			}
		}
	case map[string]interface{}:
		// An ImageObject wraps the URL.
		for _, k := range []string{"url", "contentUrl"} {
			if s := str(t[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

func brandName(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]interface{}:
		return str(t["name"])
	}
	return ""
}

func stringExtra(d Draft, key string) string {
	if d.Extras == nil {
		return ""
	}
	s, _ := d.Extras[key].(string)
	return s
}

// cleanTitle strips stray markup, unescapes entities, and collapses whitespace.
func cleanTitle(s string) string {
	s = tagStripper.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// siteSuffixSeparators are the characters retailers use to bolt their brand onto a title.
var siteSuffixSeparators = []string{" | ", " - ", " – ", " — ", " :: "}

// trimSiteSuffix removes a trailing store name from a page title. It only trims the last
// segment, and only when that segment is short, so real product names containing a dash
// ("X-Mid 1 - Solid Inner") survive.
func trimSiteSuffix(name string) string {
	for _, sep := range siteSuffixSeparators {
		if idx := strings.LastIndex(name, sep); idx > 0 {
			head, tail := name[:idx], name[idx+len(sep):]
			if len(tail) <= 24 && len(head) >= 8 && !strings.ContainsAny(tail, "0123456789") {
				return strings.TrimSpace(head)
			}
		}
	}
	return name
}
