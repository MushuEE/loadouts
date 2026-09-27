package core

import (
	"fmt"
	"sort"
	"strings"
)

// Tags are free-form labels a profile puts on its gear (#warmwear, #coldwear, #ultralight)
// so one loadout can hold several variants of a kit and be filtered down to one of them.
//
// A tag belongs to a profile, never to an item: "#warmwear" on a Capilene is one person's
// opinion of it, and two people can disagree. So tags are kept outside the metadata layers
// (where a global or community layer could merge in tags nobody chose) and attached to a
// ResolvedItem as the tags of whoever the item is being resolved for. What the site knows
// about an item's tags globally is an aggregate over every profile's, never a stored value.

const (
	maxTagLength   = 32
	maxTagsPerItem = 16
)

// NormalizeTag canonicalises a tag as typed: a leading '#' is dropped, it is lowercased,
// and spaces become hyphens. It returns an error for anything left that is not a
// lowercase letter, digit, '-' or '_'.
func NormalizeTag(raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(raw))
	t = strings.TrimLeft(t, "#")
	t = strings.Join(strings.Fields(t), "-")
	if t == "" {
		return "", fmt.Errorf("%w: a tag cannot be empty", ErrInvalid)
	}
	if len(t) > maxTagLength {
		return "", fmt.Errorf("%w: tag %q is longer than %d characters", ErrInvalid, t, maxTagLength)
	}
	for _, r := range t {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", fmt.Errorf("%w: tag %q may only use letters, digits, '-' and '_'", ErrInvalid, t)
		}
	}
	return t, nil
}

// NormalizeTags normalises and de-duplicates a list, keeping first-seen order.
func NormalizeTags(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		t, err := NormalizeTag(r)
		if err != nil {
			return nil, err
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) > maxTagsPerItem {
		return nil, fmt.Errorf("%w: an item can carry at most %d tags", ErrInvalid, maxTagsPerItem)
	}
	return out, nil
}

// TagFilter narrows a loadout to the gear carrying any of Tags.
//
// Untagged gear is kept unless ExcludeUntagged is set. The common case is one loadout
// holding a warm and a cold variant of a kit: the tent and the pack belong to both and
// nobody wants to tag them twice, so by default turning on #warmwear hides only the gear
// tagged for something else.
type TagFilter struct {
	Tags            []string `json:"tags"`
	ExcludeUntagged bool     `json:"exclude_untagged"`
}

// Active reports whether the filter hides anything at all.
func (f TagFilter) Active() bool { return len(f.Tags) > 0 }

// Matches reports whether an item passes the filter on its own merits.
func (f TagFilter) Matches(item ResolvedItem) bool {
	if !f.Active() {
		return true
	}
	tags := item.Tags
	if len(tags) == 0 {
		return !f.ExcludeUntagged
	}
	for _, t := range tags {
		for _, want := range f.Tags {
			if t == want {
				return true
			}
		}
	}
	return false
}

// TagCount is one tag present in a loadout and how many entries carry it.
// Elsewhere it is the number of profiles using the tag.
type TagCount struct {
	Tag   string `json:"tag" db:"tag"`
	Count int    `json:"count" db:"count"`
}

// ItemTags is one item's tags as a profile sees them: its own, and the aggregate of
// everyone's.
type ItemTags struct {
	Mine   []string   `json:"mine"`
	Global []TagCount `json:"global"`
}

// LoadoutFilter describes the filter a LoadoutDetail was built with. It is absent when no
// filter was requested, apart from AvailableTags, which the client needs either way to
// offer the chips.
type LoadoutFilter struct {
	TagFilter
	// AvailableTags lists every tag in the whole loadout, filtered or not, so turning a
	// tag on never makes the others disappear from the bar.
	AvailableTags []TagCount `json:"available_tags"`
	// ShownEntries and TotalEntries count entries, nesting included.
	ShownEntries int `json:"shown_entries"`
	TotalEntries int `json:"total_entries"`
}

// ApplyTagFilter marks entries the filter hides, in place, and returns the entry IDs it
// kept.
//
// An entry is kept if it matches or if anything inside it is kept, so a match is never
// buried inside a hidden container: an untagged pack stays visible while it holds
// something you asked to see. Hidden entries stay in the tree, flagged, because the client
// writes the whole tree back when it edits, and dropping them here would delete them.
func ApplyTagFilter(entries []ResolvedEntry, f TagFilter) map[string]bool {
	kept := map[string]bool{}
	var walk func(nodes []ResolvedEntry) bool
	walk = func(nodes []ResolvedEntry) bool {
		anyKept := false
		for i := range nodes {
			n := &nodes[i]
			childKept := walk(n.Children)
			keep := f.Matches(n.Item) || childKept
			n.Hidden = !keep
			if keep {
				kept[n.Entry.ID] = true
				anyKept = true
			}
		}
		return anyKept
	}
	walk(entries)
	return kept
}

// CountTags tallies tags across a tree, most used first and alphabetical among equals.
func CountTags(entries []ResolvedEntry) []TagCount {
	counts := map[string]int{}
	var walk func(nodes []ResolvedEntry)
	walk = func(nodes []ResolvedEntry) {
		for _, n := range nodes {
			for _, t := range n.Item.Tags {
				counts[t]++
			}
			walk(n.Children)
		}
	}
	walk(entries)
	return SortTagCounts(counts, 0)
}

// SortTagCounts turns a tally into a list, most used first and alphabetical among equals,
// cut to limit when limit is positive.
func SortTagCounts(counts map[string]int, limit int) []TagCount {
	out := make([]TagCount, 0, len(counts))
	for t, c := range counts {
		out = append(out, TagCount{Tag: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
