package core

// GearScope is the one switch between searching your own gear and everyone's.
//
// "Mine" is the gear you have a relationship with: used in one of your loadouts, tagged,
// annotated in your layer, or imported by you. Tags then mean your tags. "Everyone" is the
// whole catalogue, and a tag matches if anyone uses it on the item.
type GearScope string

const (
	ScopeMine     GearScope = "mine"
	ScopeEveryone GearScope = "everyone"
)

// ParseGearScope reads a scope from a query string, defaulting to everyone.
func ParseGearScope(raw string) GearScope {
	if GearScope(raw) == ScopeMine {
		return ScopeMine
	}
	return ScopeEveryone
}

// GearQuery searches items. Tags narrow: an item must carry every one of them, because a
// search is how you pin down "hats I wore in Seattle" rather than widen to "hats or
// Seattle". (A loadout's filter bar is the opposite, a union, since it switches between
// variants of one kit.)
type GearQuery struct {
	Text      string
	Category  string
	Tags      []string
	Scope     GearScope
	ProfileID string // The searcher. Required for ScopeMine.
	Limit     int
}

// GearResult is one item in a search, with its tags from both points of view.
type GearResult struct {
	Item Item `json:"item"`
	// Mine reports whether the item is in the searcher's gear, whatever the scope.
	Mine bool `json:"mine"`
	// MyTags are the searcher's tags; Tags are everyone's, counted in profiles.
	MyTags []string   `json:"my_tags"`
	Tags   []TagCount `json:"tags"`
}

// CategoryCount is how many results fall in one category.
type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// GearSearch is a page of results plus the facets for narrowing it further.
type GearSearch struct {
	Scope   GearScope    `json:"scope"`
	Results []GearResult `json:"results"`
	// TagFacets are the tags present in the results (counted in items), in the scope's
	// sense of tag: yours under "mine", anyone's under "everyone".
	TagFacets []TagCount `json:"tag_facets"`
	// CategoryFacets ignore the category filter, so picking one never hides the others.
	CategoryFacets []CategoryCount `json:"category_facets"`
	Total          int             `json:"total"`
}
