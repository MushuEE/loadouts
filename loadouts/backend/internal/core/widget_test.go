package core

import (
	"strings"
	"testing"
)

// widgetFixture is a three-entry loadout: two shelter items and one sleep item.
//
//	Tarp   240 g x1 = 240 g
//	Stake   10 g x2 =  20 g
//	Quilt  560 g x1 = 560 g
//	                  ------
//	                  820 g
func widgetFixture() WidgetData {
	row := func(name, category string, grams, cents, qty float64) map[string]interface{} {
		return map[string]interface{}{
			"entry": map[string]interface{}{"quantity": qty},
			"item": map[string]interface{}{
				"name":     name,
				"category": category,
				"metadata": map[string]interface{}{
					"core": map[string]interface{}{"weight_g": grams, "cost_cents": cents},
				},
			},
		}
	}
	return WidgetData{
		Rows: []map[string]interface{}{
			row("Tarp", "shelter", 240, 12000, 1),
			row("Stake", "shelter", 10, 500, 2),
			row("Quilt", "sleep", 560, 32000, 1),
		},
		Context: map[string]interface{}{
			"loadout": map[string]interface{}{
				"name":  "UL Kit",
				"stats": map[string]interface{}{"total_weight_g": 820.0},
			},
		},
	}
}

const totalWeightExpr = "item.metadata.core.weight_g * entry.quantity"

// --- Validation ---

func TestValidateWidgetSpecAcceptsGoodSpecs(t *testing.T) {
	specs := map[string]WidgetSpec{
		"stat grid": {
			Type:   WidgetStatGrid,
			Source: SourceLoadoutEntries,
			Stats: []WidgetStat{
				{Label: "Total", Value: "sum(" + totalWeightExpr + ")", Format: FormatGrams},
			},
		},
		"table": {
			Type:    WidgetTable,
			Source:  SourceLoadoutEntries,
			Columns: []WidgetColumn{{Label: "Name", Value: "item.name", Format: FormatText}},
			SortBy:  "item.name",
		},
		"bar chart": {
			Type:    WidgetBarChart,
			Source:  SourceLoadoutEntries,
			Label:   "item.category",
			Value:   "sum(" + totalWeightExpr + ")",
			GroupBy: "item.category",
			SortBy:  "value",
		},
		"pie chart with a filter": {
			Type:   WidgetPieChart,
			Source: SourceLoadoutEntries,
			Label:  "item.name",
			Value:  "item.metadata.core.cost_cents",
			Filter: "item.metadata.core.cost_cents > 0",
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			if err := ValidateWidgetSpec(spec); err != nil {
				t.Fatalf("ValidateWidgetSpec rejected a valid spec: %v", err)
			}
		})
	}
}

func TestValidateWidgetSpecRejectsBadSpecs(t *testing.T) {
	cases := []struct {
		name string
		spec WidgetSpec
		want string
	}{
		{
			"unknown type",
			WidgetSpec{Type: "sparkline", Source: SourceLoadoutEntries},
			"unknown widget type",
		},
		{
			"unknown source",
			WidgetSpec{Type: WidgetTable, Source: "postgres"},
			"unknown data source",
		},
		{
			"stat grid with no stats",
			WidgetSpec{Type: WidgetStatGrid, Source: SourceLoadoutEntries},
			"at least one stat",
		},
		{
			"stat with an unparseable expression",
			WidgetSpec{Type: WidgetStatGrid, Source: SourceLoadoutEntries,
				Stats: []WidgetStat{{Label: "Broken", Value: "sum(w"}}},
			"Broken",
		},
		{
			"stat with no label",
			WidgetSpec{Type: WidgetStatGrid, Source: SourceLoadoutEntries,
				Stats: []WidgetStat{{Value: "1"}}},
			"no label",
		},
		{
			"unknown format",
			WidgetSpec{Type: WidgetStatGrid, Source: SourceLoadoutEntries,
				Stats: []WidgetStat{{Label: "X", Value: "1", Format: "furlongs"}}},
			"unknown format",
		},
		{
			"table with no columns",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries},
			"at least one column",
		},
		{
			"column with a bad align",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries,
				Columns: []WidgetColumn{{Label: "X", Value: "1", Align: "sideways"}}},
			"unknown align",
		},
		{
			"table sorted by nonsense",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries,
				Columns: []WidgetColumn{{Label: "X", Value: "1"}}, SortBy: "1 +"},
			"sort_by",
		},
		{
			"table over the row ceiling",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries,
				Columns: []WidgetColumn{{Label: "X", Value: "1"}}, Limit: 10000},
			"at most",
		},
		{
			"chart with no label",
			WidgetSpec{Type: WidgetBarChart, Source: SourceLoadoutEntries, Value: "1"},
			"label expression",
		},
		{
			"chart with no value",
			WidgetSpec{Type: WidgetPieChart, Source: SourceLoadoutEntries, Label: "item.name"},
			"value expression",
		},
		{
			"chart sorted by an expression",
			WidgetSpec{Type: WidgetBarChart, Source: SourceLoadoutEntries,
				Label: "item.name", Value: "1", SortBy: "item.name"},
			"sorts by",
		},
		{
			"chart grouped by an aggregate",
			WidgetSpec{Type: WidgetBarChart, Source: SourceLoadoutEntries,
				Label: "item.name", Value: "1", GroupBy: "sum(item.metadata.core.weight_g)"},
			"cannot aggregate",
		},
		{
			"unparseable filter",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries,
				Columns: []WidgetColumn{{Label: "X", Value: "1"}}, Filter: "&&"},
			"filter",
		},
		{
			"negative limit",
			WidgetSpec{Type: WidgetTable, Source: SourceLoadoutEntries,
				Columns: []WidgetColumn{{Label: "X", Value: "1"}}, Limit: -1},
			"negative",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWidgetSpec(tc.spec)
			if err == nil {
				t.Fatal("ValidateWidgetSpec accepted an invalid spec")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// --- Stat grid ---

func TestEvaluateStatGrid(t *testing.T) {
	spec := WidgetSpec{
		Type:   WidgetStatGrid,
		Source: SourceLoadoutEntries,
		Stats: []WidgetStat{
			{Label: "Total weight", Value: "sum(" + totalWeightExpr + ")", Format: FormatGrams},
			{Label: "Entries", Value: "count(1)", Format: FormatInteger},
			{Label: "Heaviest", Value: "max(item.metadata.core.weight_g)", Format: FormatGrams},
			{Label: "Spend", Value: "sum(item.metadata.core.cost_cents * entry.quantity)", Format: FormatCents},
			// Context is readable from a stat even though no row is in scope.
			{Label: "From context", Value: "loadout.stats.total_weight_g", Format: FormatGrams},
		},
	}
	if err := ValidateWidgetSpec(spec); err != nil {
		t.Fatalf("spec did not validate: %v", err)
	}

	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if render.Empty {
		t.Fatal("render reported empty, want populated")
	}

	want := []string{"820 g", "3", "560 g", "$450.00", "820 g"}
	if len(render.Stats) != len(want) {
		t.Fatalf("got %d stats, want %d", len(render.Stats), len(want))
	}
	for i, w := range want {
		if render.Stats[i].Display != w {
			t.Errorf("stat %q display = %q, want %q", render.Stats[i].Label, render.Stats[i].Display, w)
		}
	}
}

// --- Table ---

func TestEvaluateTableSortsAndComputesShares(t *testing.T) {
	spec := WidgetSpec{
		Type:   WidgetTable,
		Source: SourceLoadoutEntries,
		Columns: []WidgetColumn{
			{Label: "Item", Value: "item.name", Format: FormatText},
			{Label: "Weight", Value: totalWeightExpr, Format: FormatGrams},
			{Label: "Share", Value: "percent(" + totalWeightExpr + ", sum(" + totalWeightExpr + "))", Format: FormatPercent},
		},
		SortBy:   totalWeightExpr,
		SortDesc: true,
	}
	if err := ValidateWidgetSpec(spec); err != nil {
		t.Fatalf("spec did not validate: %v", err)
	}

	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}

	if got := len(render.Columns); got != 3 {
		t.Fatalf("got %d columns, want 3", got)
	}
	// Align defaults by format: text left, numbers right.
	if render.Columns[0].Align != "left" || render.Columns[1].Align != "right" {
		t.Errorf("column alignment = %q/%q, want left/right", render.Columns[0].Align, render.Columns[1].Align)
	}

	want := [][]string{
		{"Quilt", "560 g", "68.3%"},
		{"Tarp", "240 g", "29.3%"},
		{"Stake", "20 g", "2.4%"},
	}
	if len(render.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(render.Rows), len(want))
	}
	for i, wantRow := range want {
		for j, wantCell := range wantRow {
			if got := render.Rows[i][j].Display; got != wantCell {
				t.Errorf("row %d column %d = %q, want %q", i, j, got, wantCell)
			}
		}
	}
	if render.Truncated {
		t.Error("render reported truncation, want none")
	}
}

func TestEvaluateTableLimitDoesNotDistortAggregates(t *testing.T) {
	// Aggregates range over the whole filtered set, not the truncated page, so a
	// "percent of total" column stays honest when rows are cut off.
	spec := WidgetSpec{
		Type:   WidgetTable,
		Source: SourceLoadoutEntries,
		Columns: []WidgetColumn{
			{Label: "Item", Value: "item.name", Format: FormatText},
			{Label: "Share", Value: "percent(" + totalWeightExpr + ", sum(" + totalWeightExpr + "))", Format: FormatPercent},
		},
		SortBy:   totalWeightExpr,
		SortDesc: true,
		Limit:    1,
	}
	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(render.Rows))
	}
	if !render.Truncated {
		t.Error("render did not report truncation, want it reported")
	}
	if got := render.Rows[0][1].Display; got != "68.3%" {
		t.Errorf("share = %q, want 68.3%% computed against all three rows", got)
	}
}

func TestEvaluateTableFilter(t *testing.T) {
	spec := WidgetSpec{
		Type:    WidgetTable,
		Source:  SourceLoadoutEntries,
		Columns: []WidgetColumn{{Label: "Item", Value: "item.name", Format: FormatText}},
		Filter:  "item.category == 'shelter'",
		SortBy:  "item.name",
	}
	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 shelter rows", len(render.Rows))
	}
	if render.Rows[0][0].Display != "Stake" || render.Rows[1][0].Display != "Tarp" {
		t.Errorf("rows = %q/%q, want Stake/Tarp", render.Rows[0][0].Display, render.Rows[1][0].Display)
	}
}

func TestEvaluateTableWithNoRowsIsEmpty(t *testing.T) {
	spec := WidgetSpec{
		Type:    WidgetTable,
		Source:  SourceLoadoutEntries,
		Columns: []WidgetColumn{{Label: "Item", Value: "item.name", Format: FormatText}},
	}
	render, err := EvaluateWidget(spec, WidgetData{})
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if !render.Empty {
		t.Error("render is not empty, want empty")
	}
	if len(render.Rows) != 0 {
		t.Errorf("got %d rows, want none", len(render.Rows))
	}
	// Headers survive an empty result so the table still explains itself.
	if len(render.Columns) != 1 {
		t.Errorf("got %d columns, want the header to survive", len(render.Columns))
	}
}

// --- Charts ---

func TestEvaluateChartGroupsWithAnAggregate(t *testing.T) {
	spec := WidgetSpec{
		Type:     WidgetPieChart,
		Source:   SourceLoadoutEntries,
		GroupBy:  "item.category",
		Label:    "item.category",
		Value:    "sum(" + totalWeightExpr + ")",
		Format:   FormatGrams,
		SortBy:   "value",
		SortDesc: true,
	}
	if err := ValidateWidgetSpec(spec); err != nil {
		t.Fatalf("spec did not validate: %v", err)
	}

	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Points) != 2 {
		t.Fatalf("got %d points, want 2 categories", len(render.Points))
	}
	if render.Points[0].Label != "sleep" || render.Points[0].Value != 560 {
		t.Errorf("first point = %q/%v, want sleep/560", render.Points[0].Label, render.Points[0].Value)
	}
	if render.Points[1].Label != "shelter" || render.Points[1].Value != 260 {
		t.Errorf("second point = %q/%v, want shelter/260", render.Points[1].Label, render.Points[1].Value)
	}
	if render.Total != 820 {
		t.Errorf("total = %v, want 820", render.Total)
	}
	if render.Points[0].Display != "560 g" {
		t.Errorf("display = %q, want 560 g", render.Points[0].Display)
	}

	var shares float64
	for _, p := range render.Points {
		shares += p.Share
	}
	if shares < 0.999 || shares > 1.001 {
		t.Errorf("shares sum to %v, want 1", shares)
	}
}

func TestEvaluateChartGroupsAPerRowValueBySumming(t *testing.T) {
	// A non-aggregate value expression under group_by is summed, which is what GROUP BY
	// means to anyone who has written SQL.
	spec := WidgetSpec{
		Type:    WidgetBarChart,
		Source:  SourceLoadoutEntries,
		GroupBy: "item.category",
		Label:   "upper(item.category)",
		Value:   totalWeightExpr,
	}
	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Points) != 2 {
		t.Fatalf("got %d points, want 2", len(render.Points))
	}
	// Unsorted, groups keep first-seen order so the chart is stable between renders.
	if render.Points[0].Label != "SHELTER" || render.Points[0].Value != 260 {
		t.Errorf("first point = %q/%v, want SHELTER/260", render.Points[0].Label, render.Points[0].Value)
	}
	if render.Points[1].Label != "SLEEP" || render.Points[1].Value != 560 {
		t.Errorf("second point = %q/%v, want SLEEP/560", render.Points[1].Label, render.Points[1].Value)
	}
}

func TestEvaluateChartWithoutGroupingIsOnePointPerRow(t *testing.T) {
	spec := WidgetSpec{
		Type:   WidgetBarChart,
		Source: SourceLoadoutEntries,
		Label:  "item.name",
		Value:  "item.metadata.core.weight_g",
	}
	render, err := EvaluateWidget(spec, widgetFixture())
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Points) != 3 {
		t.Fatalf("got %d points, want 3", len(render.Points))
	}
	if render.Points[0].Label != "Tarp" || render.Points[1].Label != "Stake" {
		t.Errorf("labels = %q/%q, want row order Tarp/Stake", render.Points[0].Label, render.Points[1].Label)
	}
}

func TestEvaluateChartLabelsMissingValues(t *testing.T) {
	// An item with no category still has to appear somewhere, rather than collapsing
	// into a nameless slice.
	data := WidgetData{Rows: []map[string]interface{}{
		{"item": map[string]interface{}{"name": "Mystery", "weight": 100.0}},
	}}
	spec := WidgetSpec{
		Type:   WidgetPieChart,
		Source: SourceLoadoutEntries,
		Label:  "item.category",
		Value:  "item.weight",
	}
	render, err := EvaluateWidget(spec, data)
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if len(render.Points) != 1 || render.Points[0].Label != "Unspecified" {
		t.Fatalf("points = %+v, want a single Unspecified point", render.Points)
	}
}

func TestEvaluateChartWithNoRowsIsEmpty(t *testing.T) {
	spec := WidgetSpec{
		Type:      WidgetPieChart,
		Source:    SourceLoadoutEntries,
		Label:     "item.name",
		Value:     "item.metadata.core.weight_g",
		EmptyText: "Add gear to see a breakdown",
	}
	render, err := EvaluateWidget(spec, WidgetData{})
	if err != nil {
		t.Fatalf("EvaluateWidget failed: %v", err)
	}
	if !render.Empty {
		t.Error("render is not empty, want empty")
	}
	if render.EmptyText != "Add gear to see a breakdown" {
		t.Errorf("empty text = %q, want it carried through", render.EmptyText)
	}
}

func TestEvaluateWidgetRejectsUnknownType(t *testing.T) {
	_, err := EvaluateWidget(WidgetSpec{Type: "sparkline"}, widgetFixture())
	if err == nil {
		t.Fatal("EvaluateWidget accepted an unknown type")
	}
}

// --- Formatting ---

func TestFormatWidgetValue(t *testing.T) {
	cases := []struct {
		name   string
		value  interface{}
		format WidgetFormat
		want   string
	}{
		{"null renders as a dash", nil, FormatGrams, "—"},
		{"grams group thousands", 1240.0, FormatGrams, "1,240 g"},
		{"grams round", 1240.6, FormatGrams, "1,241 g"},
		{"kilograms", 1240.0, FormatKilos, "1.24 kg"},
		{"kilograms trim zeros", 2000.0, FormatKilos, "2 kg"},
		{"ounces", 28.349523125, FormatOunces, "1 oz"},
		{"pounds", 453.59237, FormatPounds, "1 lb"},
		{"cents", 12400.0, FormatCents, "$124.00"},
		{"cents group thousands", 1234567.0, FormatCents, "$12,345.67"},
		{"percent", 12.34, FormatPercent, "12.3%"},
		{"percent trims a whole number", 50.0, FormatPercent, "50%"},
		{"integer rounds", 1234.6, FormatInteger, "1,235"},
		{"integer negative", -1234.0, FormatInteger, "-1,234"},
		{"number keeps two decimals", 1234.5, FormatNumber, "1,234.5"},
		{"boolean true", true, FormatBool, "Yes"},
		{"boolean false", 0.0, FormatBool, "No"},
		{"text passes through", "Tarp", FormatText, "Tarp"},
		{"duration under an hour", 45.0, FormatDuration, "45 min"},
		{"duration exact hours", 120.0, FormatDuration, "2 h"},
		{"duration mixed", 90.0, FormatDuration, "1 h 30 min"},
		{"auto keeps a string", "Tarp", FormatAuto, "Tarp"},
		{"auto trims a whole number", 2.0, FormatAuto, "2"},
		{"auto keeps decimals", 2.5, FormatAuto, "2.5"},
		{"auto on a bool", false, FormatAuto, "No"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatWidgetValue(tc.value, tc.format); got != tc.want {
				t.Fatalf("FormatWidgetValue(%#v, %q) = %q, want %q", tc.value, tc.format, got, tc.want)
			}
		})
	}
}

// --- Manifest integration ---

func TestValidateManifestCompilesWidgetExpressions(t *testing.T) {
	// A plugin whose arithmetic does not parse must fail at publish time, not at render.
	manifest := PluginManifest{
		APIVersion: ManifestAPIVersion,
		Views: []PluginView{{
			ID:      "breakdown",
			Title:   "Weight breakdown",
			Surface: SurfaceLoadoutPanel,
			Kind:    ViewWidget,
			Widget: &WidgetSpec{
				Type:    WidgetPieChart,
				Source:  SourceLoadoutEntries,
				GroupBy: "item.category",
				Label:   "item.category",
				Value:   "sum(item.metadata.core.weight_g * ", // unbalanced
			},
		}},
	}
	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("ValidateManifest accepted a widget with a broken expression")
	}
	if !strings.Contains(err.Error(), "breakdown") {
		t.Fatalf("error = %q, want it to name the offending view", err)
	}

	manifest.Views[0].Widget.Value = "sum(item.metadata.core.weight_g * entry.quantity)"
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest rejected a valid manifest: %v", err)
	}
}
