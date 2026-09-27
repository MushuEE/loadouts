package core

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{"#WarmWear", " cold wear ", "warmwear", "ul_2025"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	want := []string{"warmwear", "cold-wear", "ul_2025"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"", "#", "rain/wind", "émoji"} {
		if _, err := NormalizeTag(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("NormalizeTag(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func tagged(id string, tags ...string) ResolvedEntry {
	return ResolvedEntry{Entry: LoadoutEntry{ID: id}, Item: ResolvedItem{ID: id, Tags: tags}}
}

func hiddenIDs(nodes []ResolvedEntry) []string {
	var out []string
	var walk func([]ResolvedEntry)
	walk = func(ns []ResolvedEntry) {
		for _, n := range ns {
			if n.Hidden {
				out = append(out, n.Entry.ID)
			}
			walk(n.Children)
		}
	}
	walk(nodes)
	return out
}

func TestApplyTagFilter(t *testing.T) {
	build := func() []ResolvedEntry {
		pack := tagged("pack")
		pack.Children = []ResolvedEntry{tagged("puffy", "coldwear"), tagged("sunhat", "warmwear")}
		coldBag := tagged("cold-bag", "coldwear")
		coldBag.Children = []ResolvedEntry{tagged("gloves", "warmwear")}
		return []ResolvedEntry{tagged("tee", "warmwear"), tagged("fleece", "coldwear"), tagged("tent"), pack, coldBag}
	}

	// Untagged gear stays: it belongs to every variant of the kit.
	tree := build()
	kept := ApplyTagFilter(tree, TagFilter{Tags: []string{"warmwear"}})
	if got := hiddenIDs(tree); !reflect.DeepEqual(got, []string{"fleece", "puffy"}) {
		t.Errorf("hidden = %v, want [fleece puffy]", got)
	}
	// A container tagged for the other variant still shows while it holds a match, so the
	// match is never buried.
	if !kept["cold-bag"] || !kept["gloves"] {
		t.Errorf("cold-bag holding warmwear gloves should be kept, kept = %v", kept)
	}

	// Strict mode drops untagged gear, but not an untagged container around a match.
	tree = build()
	ApplyTagFilter(tree, TagFilter{Tags: []string{"warmwear"}, ExcludeUntagged: true})
	if got := hiddenIDs(tree); !reflect.DeepEqual(got, []string{"fleece", "tent", "puffy"}) {
		t.Errorf("strict hidden = %v, want [fleece tent puffy]", got)
	}

	// Several tags widen the filter rather than narrowing it.
	tree = build()
	ApplyTagFilter(tree, TagFilter{Tags: []string{"warmwear", "coldwear"}, ExcludeUntagged: true})
	if got := hiddenIDs(tree); !reflect.DeepEqual(got, []string{"tent"}) {
		t.Errorf("union hidden = %v, want [tent]", got)
	}

	counts := CountTags(build())
	want := []TagCount{{"coldwear", 3}, {"warmwear", 3}}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("CountTags = %v, want %v", counts, want)
	}
}
