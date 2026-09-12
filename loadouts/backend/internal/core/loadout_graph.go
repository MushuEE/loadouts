package core

import "fmt"

// The loadout reference graph.
//
// A loadout may occupy a slot of another loadout — "Meals Day 1" hanging off a trip's food
// slot — which turns loadouts into a directed graph rather than a set of flat lists. Two
// properties have to hold for that graph, and both are enforced when an edge is written
// rather than when it is read:
//
//  1. It is acyclic. A depth cap alone does not make A -> B -> A safe; it just bounds how
//     long a reader spins before giving up, and it would leave the cycle in the database
//     as a permanent trap for every future reader.
//  2. No chain exceeds MaxLoadoutDepth loadouts.
//
// Everything here is pure: the caller supplies adjacency as functions, so the same logic
// is exercised by unit tests over a literal map and by the service over the store.

// MaxLoadoutDepth is the longest chain of nested loadouts allowed, counting both ends.
// A trip holding meals holding nothing is a chain of 2.
const MaxLoadoutDepth = 5

// ChildrenFunc returns the loadout IDs directly referenced by a loadout's entries.
type ChildrenFunc func(loadoutID string) ([]string, error)

// ParentsFunc returns the loadout IDs whose entries directly reference this loadout.
type ParentsFunc func(loadoutID string) ([]string, error)

// ErrGraphCycle reports a cycle discovered while walking. Reaching this from a read path
// means something already wrote a cycle, which the write path is supposed to prevent.
var ErrGraphCycle = fmt.Errorf("%w: loadout reference cycle", ErrInvalid)

// HeightBelow returns the length of the longest chain starting at id, counting id itself.
// A loadout with no sub-loadouts has height 1.
func HeightBelow(id string, children ChildrenFunc) (int, error) {
	return longestPath(id, children)
}

// DepthAbove returns the length of the longest chain ending at id, counting id itself.
// A loadout nothing references has depth 1.
func DepthAbove(id string, parents ParentsFunc) (int, error) {
	return longestPath(id, ChildrenFunc(parents))
}

// longestPath walks one direction of the graph, memoizing so that a diamond is not
// re-explored and a wide graph does not go exponential.
//
// The state map doubles as cycle detection: a node still being visited when we reach it
// again is a back edge.
func longestPath(start string, next ChildrenFunc) (int, error) {
	const (
		visiting = -1
	)
	best := map[string]int{}

	var walk func(id string) (int, error)
	walk = func(id string) (int, error) {
		if n, seen := best[id]; seen {
			if n == visiting {
				return 0, fmt.Errorf("%w at %s", ErrGraphCycle, id)
			}
			return n, nil
		}
		best[id] = visiting

		adjacent, err := next(id)
		if err != nil {
			return 0, err
		}
		longest := 0
		for _, a := range adjacent {
			n, err := walk(a)
			if err != nil {
				return 0, err
			}
			if n > longest {
				longest = n
			}
		}
		best[id] = longest + 1
		return best[id], nil
	}
	return walk(start)
}

// Reaches reports whether target is from, or is reachable from it by following references.
func Reaches(from, target string, children ChildrenFunc) (bool, error) {
	if from == target {
		return true, nil
	}
	seen := map[string]bool{}

	var walk func(id string) (bool, error)
	walk = func(id string) (bool, error) {
		if seen[id] {
			return false, nil
		}
		seen[id] = true

		adjacent, err := children(id)
		if err != nil {
			return false, err
		}
		for _, a := range adjacent {
			if a == target {
				return true, nil
			}
			found, err := walk(a)
			if err != nil || found {
				return found, err
			}
		}
		return false, nil
	}
	return walk(from)
}

// ValidateAttachment reports whether childID may be placed into a slot of parentID.
//
// The depth check has to look in both directions. Hanging a 3-deep subtree off something
// that is already 3 deep breaks the invariant for every loadout *above* the parent, not
// just for the parent, so the test is over the whole chain the new edge would create:
//
//	depthAbove(parent) + heightBelow(child) <= MaxLoadoutDepth
func ValidateAttachment(parentID, childID string, children ChildrenFunc, parents ParentsFunc) error {
	if parentID == "" || childID == "" {
		return fmt.Errorf("%w: both a parent and a child loadout are required", ErrInvalid)
	}
	if parentID == childID {
		return fmt.Errorf("%w: a loadout cannot contain itself", ErrInvalid)
	}

	// Would this close a loop? It does exactly when the parent is already somewhere
	// beneath the child.
	loops, err := Reaches(childID, parentID, children)
	if err != nil {
		return err
	}
	if loops {
		return fmt.Errorf("%w: that would create a loop, because this loadout is already inside the one you are adding", ErrInvalid)
	}

	above, err := DepthAbove(parentID, parents)
	if err != nil {
		return err
	}
	below, err := HeightBelow(childID, children)
	if err != nil {
		return err
	}
	if total := above + below; total > MaxLoadoutDepth {
		return fmt.Errorf("%w: nesting that would be %d loadouts deep, and the limit is %d",
			ErrInvalid, total, MaxLoadoutDepth)
	}
	return nil
}

// ValidateSlots checks a template's slot list for internal contradictions.
func ValidateSlots(slots SlotList) error {
	seen := map[string]bool{}
	for _, s := range slots {
		if s.ID == "" {
			return fmt.Errorf("%w: every slot needs an id", ErrInvalid)
		}
		if seen[s.ID] {
			return fmt.Errorf("%w: duplicate slot id %q", ErrInvalid, s.ID)
		}
		seen[s.ID] = true

		// A slot that takes both items and loadouts leaves the UI with no answer to
		// "what goes here?", and leaves the validator with no rule to apply.
		if s.HoldsSubLoadouts() && len(s.AcceptedCategories) > 0 {
			return fmt.Errorf("%w: slot %q accepts both item categories and templates; it must be one or the other",
				ErrInvalid, s.ID)
		}
		if !s.Selection.IsValid() {
			return fmt.Errorf("%w: slot %q has unknown selection mode %q", ErrInvalid, s.ID, s.Selection)
		}
		if s.Selection == SelectionAlternatives && s.Capacity() == 1 {
			return fmt.Errorf("%w: slot %q is a single-capacity slot, so there is nothing to choose between",
				ErrInvalid, s.ID)
		}
	}
	return nil
}
