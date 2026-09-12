package importer

// Draft is the best guess at an item, assembled from whatever the product page gave us.
//
// Every field is optional. A Draft with nothing but a Target is still useful: the user
// can fill in the rest in the preview step and the item will still link and dedupe
// correctly. Fields the extractor could not determine are left zero so the UI can tell
// "we read 0 grams off the page" apart from "we don't know the weight".
type Draft struct {
	Target Target `json:"target"`

	Name        string `json:"name"`
	Brand       string `json:"brand"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	// Category is a guess mapped onto the template slot vocabulary; may be "".
	Category string `json:"category"`

	// WeightG is grams. HasWeight distinguishes "unknown" from "zero".
	WeightG    float64 `json:"weight_g"`
	HasWeight  bool    `json:"has_weight"`
	CostCents  int64   `json:"cost_cents"`
	HasPrice   bool    `json:"has_price"`
	Currency   string  `json:"currency"`
	Consumable bool    `json:"consumable"`

	// Extras holds anything interesting we scraped that has no first-class home,
	// e.g. sku, gtin, color. It lands in the item's "import" metadata namespace.
	Extras map[string]interface{} `json:"extras,omitempty"`

	// Source records which extraction strategy produced this draft, for debugging a
	// retailer whose markup has changed.
	Source string `json:"extracted_via,omitempty"`
}

// Extraction strategies, most to least trustworthy.
const (
	ViaJSONLD = "json-ld"
	ViaMeta   = "meta-tags"
	ViaTitle  = "title"
	ViaNone   = "none"
)

func (d *Draft) setExtra(key string, value interface{}) {
	if value == nil || value == "" {
		return
	}
	if d.Extras == nil {
		d.Extras = map[string]interface{}{}
	}
	d.Extras[key] = value
}

// IsUsable reports whether we got enough to skip a fully manual entry form.
func (d Draft) IsUsable() bool { return d.Name != "" }
