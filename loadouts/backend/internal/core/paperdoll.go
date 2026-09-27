package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
)

// A PaperdollLayout places a template's slots on a grid.
//
// The canvas is a coarse grid of cells, roughly one hand or wrist per cell, so an author can
// paint it by hand. Blocks are dropped onto it: prefabricated figures (a runner, a hiker, a
// tent) whose smooth silhouettes the client draws, or plain rectangular tiles for things with
// no body, like a meal plan. A binding then claims a set of cells for one slot: the runner's
// torso cells for "Running shirt", a 4x4 tile for "Breakfast".
//
// The layout says *where* a slot is and nothing else. What a slot accepts stays on
// SlotDefinition, and layering (a shell over a fleece) is several items in one slot rather
// than several slots stacked on one cell. That is why a cell belongs to at most one slot.
//
// Layouts live on the TemplateVersion, so loadouts pinned to an older version keep the
// paperdoll they were built against. A version with no layout falls back to the
// category-inferred figure on the client.
type PaperdollLayout struct {
	Cols     int                `json:"cols"`
	Rows     int                `json:"rows"`
	Blocks   []PaperdollBlock   `json:"blocks"`
	Bindings []PaperdollBinding `json:"bindings"`
}

// PaperdollBlock is one figure or tile placed on the canvas, by its top-left cell.
type PaperdollBlock struct {
	Block string `json:"block"`
	Col   int    `json:"col"`
	Row   int    `json:"row"`
	// Cols and Rows size a tile. Figures have a fixed footprint and ignore them.
	Cols int `json:"cols,omitempty"`
	Rows int `json:"rows,omitempty"`
}

// PaperdollBinding claims cells for a slot. Cells are run-length encoded over the canvas in
// row-major order: [[start, length], ...], where cell index = row*cols + col.
type PaperdollBinding struct {
	SlotID string   `json:"slot_id"`
	Cells  CellRuns `json:"cells"`
}

// CellRuns is a run-length encoded cell set. Runs are the natural shape of a painted
// region - a torso is a few horizontal strips - so this stays small without a bitset's
// fiddly encoding, and it is still readable in a JSON dump.
type CellRuns [][2]int

// TileBlock is the one block with no figure: a plain rectangle sized per placement.
const TileBlock = "tile"

// PaperdollFigure is a prefabricated block with a fixed footprint. The client owns the
// silhouette artwork; the server only needs the footprint to validate placements.
type PaperdollFigure struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
}

// PaperdollFigures is the seeded figure set. Placing a figure is site-admin only, so there
// is no user-supplied artwork to sanitise yet. Footprints must match the client's
// frontend/src/paperdoll/figures.tsx.
var PaperdollFigures = map[string]PaperdollFigure{
	"hiker":   {ID: "hiker", Label: "Hiker", Cols: 10, Rows: 16},
	"runner":  {ID: "runner", Label: "Runner", Cols: 10, Rows: 16},
	"cyclist": {ID: "cyclist", Label: "Cyclist", Cols: 16, Rows: 12},
	"tent":    {ID: "tent", Label: "Tent", Cols: 10, Rows: 6},
}

// MaxPaperdollSide bounds the canvas. At hand-sized cells a figure is about 16 cells tall,
// so this leaves room for several blocks while keeping a layout small enough to paint.
const MaxPaperdollSide = 48

// Footprint returns the block's size in cells.
func (b PaperdollBlock) Footprint() (cols, rows int) {
	if fig, ok := PaperdollFigures[b.Block]; ok {
		return fig.Cols, fig.Rows
	}
	return b.Cols, b.Rows
}

// contains reports whether a canvas cell falls inside this block.
func (b PaperdollBlock) contains(col, row int) bool {
	cols, rows := b.Footprint()
	return col >= b.Col && col < b.Col+cols && row >= b.Row && row < b.Row+rows
}

// HasFigures reports whether the layout places any silhouette. Those layouts are the
// site-admin-only kind; tile-only layouts are not.
func (l *PaperdollLayout) HasFigures() bool {
	if l == nil {
		return false
	}
	for _, b := range l.Blocks {
		if b.Block != TileBlock {
			return true
		}
	}
	return false
}

// Expand decodes runs into sorted cell indices, rejecting runs that leave a canvas of the
// given size.
func (r CellRuns) Expand(total int) ([]int, error) {
	var cells []int
	for _, run := range r {
		start, length := run[0], run[1]
		if length <= 0 || start < 0 || start+length > total {
			return nil, fmt.Errorf("%w: cell run [%d, %d] is outside the canvas", ErrInvalid, start, length)
		}
		for c := start; c < start+length; c++ {
			cells = append(cells, c)
		}
	}
	sort.Ints(cells)
	return cells, nil
}

// EncodeCells run-length encodes cell indices. Duplicates are ignored.
func EncodeCells(cells []int) CellRuns {
	sorted := append([]int(nil), cells...)
	sort.Ints(sorted)
	runs := CellRuns{}
	for _, c := range sorted {
		if n := len(runs); n > 0 {
			last := &runs[n-1]
			if c < last[0]+last[1] {
				continue
			}
			if c == last[0]+last[1] {
				last[1]++
				continue
			}
		}
		runs = append(runs, [2]int{c, 1})
	}
	return runs
}

// ValidatePaperdoll checks a layout against the slots of the version it will ship with.
//
// Blocks may not overlap, since a cell under two silhouettes has no single shape to clip a
// highlight to. Bindings must name a real slot, at most once, and may only claim cells that
// sit on a block and that no other binding has claimed.
func ValidatePaperdoll(l *PaperdollLayout, slots SlotList) error {
	if l == nil {
		return nil
	}
	if l.Cols < 1 || l.Rows < 1 || l.Cols > MaxPaperdollSide || l.Rows > MaxPaperdollSide {
		return fmt.Errorf("%w: paperdoll canvas must be between 1x1 and %dx%d cells", ErrInvalid, MaxPaperdollSide, MaxPaperdollSide)
	}

	for i, b := range l.Blocks {
		if b.Block != TileBlock {
			if _, ok := PaperdollFigures[b.Block]; !ok {
				return fmt.Errorf("%w: unknown paperdoll block %q", ErrInvalid, b.Block)
			}
		}
		cols, rows := b.Footprint()
		if cols < 1 || rows < 1 {
			return fmt.Errorf("%w: tile %d needs a size", ErrInvalid, i)
		}
		if b.Col < 0 || b.Row < 0 || b.Col+cols > l.Cols || b.Row+rows > l.Rows {
			return fmt.Errorf("%w: block %q does not fit on the canvas", ErrInvalid, b.Block)
		}
		for j := 0; j < i; j++ {
			if blocksOverlap(b, l.Blocks[j]) {
				return fmt.Errorf("%w: blocks %q and %q overlap", ErrInvalid, l.Blocks[j].Block, b.Block)
			}
		}
	}

	owner := map[int]string{}
	seen := map[string]bool{}
	for _, binding := range l.Bindings {
		if _, ok := slots.ByID(binding.SlotID); !ok {
			return fmt.Errorf("%w: paperdoll binds unknown slot %q", ErrInvalid, binding.SlotID)
		}
		if seen[binding.SlotID] {
			return fmt.Errorf("%w: slot %q is bound twice; put all its cells in one binding", ErrInvalid, binding.SlotID)
		}
		seen[binding.SlotID] = true

		cells, err := binding.Cells.Expand(l.Cols * l.Rows)
		if err != nil {
			return err
		}
		if len(cells) == 0 {
			return fmt.Errorf("%w: slot %q is bound to no cells", ErrInvalid, binding.SlotID)
		}
		for _, c := range cells {
			col, row := c%l.Cols, c/l.Cols
			if prev, taken := owner[c]; taken {
				return fmt.Errorf("%w: cell (%d, %d) is claimed by both %q and %q; a cell belongs to one slot", ErrInvalid, col, row, prev, binding.SlotID)
			}
			owner[c] = binding.SlotID
			if !l.onBlock(col, row) {
				return fmt.Errorf("%w: slot %q claims cell (%d, %d), which is not on any block", ErrInvalid, binding.SlotID, col, row)
			}
		}
	}
	return nil
}

func (l *PaperdollLayout) onBlock(col, row int) bool {
	for _, b := range l.Blocks {
		if b.contains(col, row) {
			return true
		}
	}
	return false
}

func blocksOverlap(a, b PaperdollBlock) bool {
	ac, ar := a.Footprint()
	bc, br := b.Footprint()
	return a.Col < b.Col+bc && b.Col < a.Col+ac && a.Row < b.Row+br && b.Row < a.Row+ar
}

// PruneBindings drops bindings for slots that are not in slots. A new version that removes
// a slot keeps the rest of its paperdoll instead of losing it or failing to publish.
func (l *PaperdollLayout) PruneBindings(slots SlotList) *PaperdollLayout {
	if l == nil {
		return nil
	}
	out := *l
	out.Blocks = append([]PaperdollBlock(nil), l.Blocks...)
	out.Bindings = nil
	for _, b := range l.Bindings {
		if _, ok := slots.ByID(b.SlotID); ok {
			out.Bindings = append(out.Bindings, b)
		}
	}
	return &out
}

func (l PaperdollLayout) Value() (driver.Value, error) {
	return json.Marshal(l)
}

func (l *PaperdollLayout) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("PaperdollLayout: type assertion to []byte failed")
	}
	return json.Unmarshal(b, l)
}
