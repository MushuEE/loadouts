package importer

import (
	"regexp"
	"strconv"
	"strings"
)

// Retailers express weight in whatever unit their marketing team prefers: "2 lb 3 oz",
// "1.2 kg", "43.5 oz", "850 g". The loadout stats engine only understands core.weight_g,
// so normalizing here is what makes an imported item immediately useful in a loadout
// rather than a row the user has to go fix by hand.

var unitToGrams = map[string]float64{
	"g": 1, "gr": 1, "gram": 1, "grams": 1,
	"kg": 1000, "kgs": 1000, "kilogram": 1000, "kilograms": 1000,
	"oz": 28.349523125, "ozs": 28.349523125, "ounce": 28.349523125, "ounces": 28.349523125,
	"lb": 453.59237, "lbs": 453.59237, "pound": 453.59237, "pounds": 453.59237,
}

// weightPattern matches a number followed by a unit, e.g. "2.5 lb" or "850g".
var weightPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*([a-z]+)\.?`)

// ParseWeightGrams converts a free-text weight into grams.
//
// Compound values are summed, so "2 lb 3 oz" yields 992.2g. Returns false when no
// recognizable weight is present, which the caller should treat as "ask the user"
// rather than "the item weighs nothing".
func ParseWeightGrams(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	// Strip thousands separators so "1,200 g" parses as 1200.
	s = strings.ReplaceAll(s, ",", "")

	var total float64
	var found bool
	for _, m := range weightPattern.FindAllStringSubmatch(s, -1) {
		unit := strings.ToLower(m[2])
		factor, ok := unitToGrams[unit]
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		total += v * factor
		found = true
	}
	if !found {
		return 0, false
	}
	return round2(total), true
}

// pricePattern matches a decimal amount, ignoring any currency symbol around it.
var pricePattern = regexp.MustCompile(`(\d+(?:\.\d{1,2})?)`)

// ParsePriceCents converts a price string ("$54.95", "54.95 USD", "1,299.00") into
// integer cents, avoiding the float rounding errors that come from storing money as a
// float and multiplying by 100 late.
func ParsePriceCents(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	m := pricePattern.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	// +0.5 before truncating avoids 54.95*100 landing on 5494.
	return int64(v*100 + 0.5), true
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

// categoryKeywords maps words that appear in product names and breadcrumbs onto the
// categories the template slot system uses. This is a guess the user can override in the
// preview step, not an authority.
var categoryKeywords = []struct {
	category string
	words    []string
}{
	{"pack", []string{"backpack", "rucksack", "daypack", "pack "}},
	{"shelter", []string{"tent", "tarp", "bivy", "shelter", "hammock"}},
	{"sleep", []string{"sleeping bag", "quilt", "sleeping pad", "air mattress", "pillow"}},
	{"kitchen", []string{"stove", "pot", "cookware", "mug", "spork", "kettle"}},
	{"fuel", []string{"fuel canister", "isopro", "canister fuel"}},
	{"shoes", []string{"shoe", "boot", "sandal", "trail runner"}},
	{"shirt", []string{"shirt", "hoody", "hoodie", "tee", "base layer", "jersey"}},
	{"jacket", []string{"jacket", "puffy", "parka", "windbreaker", "rain shell"}},
	{"pants", []string{"pant", "short", "tight", "legging", "bib"}},
	{"poles", []string{"trekking pole", "hiking pole", "ski pole"}},
	{"electronics", []string{"battery", "power bank", "headlamp", "gps", "watch", "charger"}},
	{"water", []string{"water filter", "squeeze filter", "purifier", "water bottle", "hydration"}},
}

// GuessCategory infers a slot-compatible category from free text. Returns "" when
// nothing matches, leaving the choice to the user.
func GuessCategory(text ...string) string {
	joined := strings.ToLower(strings.Join(text, " "))
	if joined == "" {
		return ""
	}
	for _, c := range categoryKeywords {
		for _, w := range c.words {
			if strings.Contains(joined, w) {
				return c.category
			}
		}
	}
	return ""
}
