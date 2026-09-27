package seed

import "github.com/gmccloskey/loadouts/backend/internal/core"

// Seed paperdolls are painted in code the way an admin paints them in the editor: as
// rectangles of cells, with the odd cell carved out. The coordinates follow the figure
// artwork in frontend/src/paperdoll/figures.tsx, where one SVG unit is one cell.

// cells is a set of canvas cells, built up from rectangles before it is encoded.
type cells struct {
	cols int
	set  map[int]bool
}

func canvas(cols int) *cells { return &cells{cols: cols, set: map[int]bool{}} }

// rect adds every cell in the inclusive column and row ranges.
func (c *cells) rect(col0, col1, row0, row1 int) *cells {
	for row := row0; row <= row1; row++ {
		for col := col0; col <= col1; col++ {
			c.set[row*c.cols+col] = true
		}
	}
	return c
}

// cut removes the same, so a region can be painted around a smaller one.
func (c *cells) cut(col0, col1, row0, row1 int) *cells {
	for row := row0; row <= row1; row++ {
		for col := col0; col <= col1; col++ {
			delete(c.set, row*c.cols+col)
		}
	}
	return c
}

func (c *cells) runs() core.CellRuns {
	list := make([]int, 0, len(c.set))
	for i := range c.set {
		list = append(list, i)
	}
	return core.EncodeCells(list)
}

func bind(slotID string, c *cells) core.PaperdollBinding {
	return core.PaperdollBinding{SlotID: slotID, Cells: c.runs()}
}

// tile places a tile and binds its whole footprint to one slot, which is what the editor
// does when an author adds a tile.
func tile(layout *core.PaperdollLayout, slotID string, col, row, w, h int) {
	layout.Blocks = append(layout.Blocks, core.PaperdollBlock{Block: core.TileBlock, Col: col, Row: row, Cols: w, Rows: h})
	layout.Bindings = append(layout.Bindings, bind(slotID, canvas(layout.Cols).rect(col, col+w-1, row, row+h-1)))
}

// backpackingPaperdoll is a hiker wearing the worn slots and carrying the pack and poles,
// the tent pitched beside them, and the sleep system as two plain tiles.
func backpackingPaperdoll() *core.PaperdollLayout {
	const cols = 21
	l := &core.PaperdollLayout{
		Cols: cols, Rows: 16,
		Blocks: []core.PaperdollBlock{
			{Block: "hiker", Col: 0, Row: 0},
			{Block: "tent", Col: 11, Row: 0},
		},
	}
	l.Bindings = []core.PaperdollBinding{
		// The pack shows above the shoulders on either side of the head.
		bind("pack", canvas(cols).rect(2, 3, 1, 2).rect(6, 7, 1, 2)),
		bind("worn-torso", canvas(cols).rect(0, 9, 3, 8)),
		bind("worn-legs", canvas(cols).rect(2, 7, 9, 13)),
		bind("worn-feet", canvas(cols).rect(2, 7, 14, 15)),
		// Poles take the hands that hold them.
		bind("poles", canvas(cols).rect(0, 1, 9, 15).rect(8, 9, 9, 15)),
		bind("shelter", canvas(cols).rect(11, 20, 0, 5)),
	}
	tile(l, "sleep-bag", 11, 7, 10, 4)
	tile(l, "sleep-pad", 11, 12, 10, 4)
	return l
}

// trailRunningPaperdoll is a runner mid-stride, with the watch on the one wrist cell that
// the front hand has to itself, and fuel and water as tiles.
func trailRunningPaperdoll() *core.PaperdollLayout {
	const cols = 16
	l := &core.PaperdollLayout{
		Cols: cols, Rows: 16,
		Blocks: []core.PaperdollBlock{{Block: "runner", Col: 0, Row: 0}},
	}
	l.Bindings = []core.PaperdollBinding{
		bind("hat", canvas(cols).rect(4, 6, 0, 1)),
		bind("shirt", canvas(cols).rect(2, 7, 3, 8).cut(7, 7, 4, 4)),
		bind("watch", canvas(cols).rect(7, 8, 4, 4)),
		bind("shorts", canvas(cols).rect(3, 7, 9, 10)),
		// The front foot is planted; the back foot is kicked up behind.
		bind("shoes", canvas(cols).rect(6, 9, 14, 15).rect(0, 2, 12, 13)),
	}
	tile(l, "nutrition", 11, 0, 5, 5)
	tile(l, "hydration", 11, 6, 5, 5)
	return l
}

// mealPlanPaperdoll has no figure at all: four tiles, one per meal. Anyone who can edit the
// template may publish a layout like this; only figures need a site admin.
func mealPlanPaperdoll() *core.PaperdollLayout {
	l := &core.PaperdollLayout{Cols: 13, Rows: 13}
	tile(l, "breakfast", 0, 0, 6, 6)
	tile(l, "lunch", 7, 0, 6, 6)
	tile(l, "dinner", 0, 7, 6, 6)
	tile(l, "snacks", 7, 7, 6, 6)
	return l
}
