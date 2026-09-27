package core

import (
	"errors"
	"reflect"
	"testing"
)

func paperdollSlots() SlotList {
	return SlotList{{ID: "shirt"}, {ID: "gloves"}, {ID: "breakfast"}}
}

func TestCellRuns_RoundTrip(t *testing.T) {
	cells := []int{7, 3, 4, 5, 20, 21, 4}
	runs := EncodeCells(cells)
	if want := (CellRuns{{3, 3}, {7, 1}, {20, 2}}); !reflect.DeepEqual(runs, want) {
		t.Fatalf("encode = %v, want %v", runs, want)
	}
	got, err := runs.Expand(100)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if want := []int{3, 4, 5, 7, 20, 21}; !reflect.DeepEqual(got, want) {
		t.Errorf("expand = %v, want %v", got, want)
	}
	if _, err := (CellRuns{{98, 3}}).Expand(100); !errors.Is(err, ErrInvalid) {
		t.Errorf("a run past the canvas should be invalid, got %v", err)
	}
}

func TestValidatePaperdoll(t *testing.T) {
	// A hiker at the origin of a 20x16 canvas, and a 4x4 tile to its right.
	base := func() *PaperdollLayout {
		return &PaperdollLayout{
			Cols: 20, Rows: 16,
			Blocks: []PaperdollBlock{
				{Block: "hiker", Col: 0, Row: 0},
				{Block: TileBlock, Col: 12, Row: 0, Cols: 4, Rows: 4},
			},
			Bindings: []PaperdollBinding{
				{SlotID: "shirt", Cells: CellRuns{{3*20 + 3, 4}}},
				// Both hands: one slot may own cells that are nowhere near each other.
				{SlotID: "gloves", Cells: CellRuns{{9 * 20, 2}, {9*20 + 8, 2}}},
				{SlotID: "breakfast", Cells: CellRuns{{12, 4}, {32, 4}, {52, 4}, {72, 4}}},
			},
		}
	}
	if err := ValidatePaperdoll(base(), paperdollSlots()); err != nil {
		t.Fatalf("valid layout rejected: %v", err)
	}

	cases := map[string]func(l *PaperdollLayout){
		"canvas too big": func(l *PaperdollLayout) { l.Cols = MaxPaperdollSide + 1 },
		"unknown figure": func(l *PaperdollLayout) { l.Blocks[0].Block = "dragon" },
		"sizeless tile":  func(l *PaperdollLayout) { l.Blocks[1].Cols = 0 },
		"off the canvas": func(l *PaperdollLayout) { l.Blocks[1].Col = 17 },
		"blocks overlap": func(l *PaperdollLayout) { l.Blocks[1].Col = 9 },
		"unknown slot":   func(l *PaperdollLayout) { l.Bindings[0].SlotID = "hat" },
		"slot bound twice": func(l *PaperdollLayout) {
			l.Bindings = append(l.Bindings, PaperdollBinding{SlotID: "shirt", Cells: CellRuns{{100, 1}}})
		},
		"no cells":       func(l *PaperdollLayout) { l.Bindings[0].Cells = CellRuns{} },
		"shared cell":    func(l *PaperdollLayout) { l.Bindings[1].Cells = CellRuns{{3*20 + 4, 1}} },
		"cell off block": func(l *PaperdollLayout) { l.Bindings[0].Cells = CellRuns{{10*20 + 15, 1}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			l := base()
			mutate(l)
			if err := ValidatePaperdoll(l, paperdollSlots()); !errors.Is(err, ErrInvalid) {
				t.Errorf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestPaperdoll_HasFiguresAndPrune(t *testing.T) {
	tiles := &PaperdollLayout{Cols: 4, Rows: 4, Blocks: []PaperdollBlock{{Block: TileBlock, Cols: 4, Rows: 4}}}
	if tiles.HasFigures() {
		t.Error("a tile-only layout has no figures")
	}
	withFigure := &PaperdollLayout{Cols: 10, Rows: 16, Blocks: []PaperdollBlock{{Block: "runner"}},
		Bindings: []PaperdollBinding{{SlotID: "shirt"}, {SlotID: "gone"}}}
	if !withFigure.HasFigures() {
		t.Error("a runner is a figure")
	}
	pruned := withFigure.PruneBindings(SlotList{{ID: "shirt"}})
	if len(pruned.Bindings) != 1 || pruned.Bindings[0].SlotID != "shirt" {
		t.Errorf("prune kept %v", pruned.Bindings)
	}
	if len(withFigure.Bindings) != 2 {
		t.Error("prune must not modify the original layout")
	}
}
