package core

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// The declarative widget tier.
//
// A WidgetSpec is a JSON description of a chart, table, or stat grid: what data to read,
// how to filter it, and what expressions produce each number. The host evaluates it
// server-side and hands the frontend a WidgetRender containing nothing but literal values
// and display strings.
//
// That asymmetry is the entire point. Plugin authors are untrusted, so a widget author
// never gets to ship code: they ship a spec, we evaluate it, and the browser only ever
// receives data. A malformed expression is a publish-time 400, not a broken page.

// WidgetType selects which host component draws the result.
type WidgetType string

const (
	WidgetStatGrid WidgetType = "stat_grid"
	WidgetBarChart WidgetType = "bar_chart"
	WidgetPieChart WidgetType = "pie_chart"
	WidgetTable    WidgetType = "table"
)

// WidgetSource names the record set a widget iterates.
type WidgetSource string

const (
	// SourceLoadoutEntries yields one row per resolved loadout entry.
	SourceLoadoutEntries WidgetSource = "loadout.entries"
	// SourceLoadoutStats yields no rows; only the loadout context is available, which
	// suits a stat grid that only reads precomputed totals.
	SourceLoadoutStats WidgetSource = "loadout.stats"
	// SourceCommunityItems yields one row per item in the community catalog.
	SourceCommunityItems WidgetSource = "community.items"
	// SourceItem yields no rows; the single item under inspection is in context.
	SourceItem WidgetSource = "item"
)

// WidgetSources lists every data source the host knows how to build.
func WidgetSources() []WidgetSource {
	return []WidgetSource{SourceLoadoutEntries, SourceLoadoutStats, SourceCommunityItems, SourceItem}
}

// IsValid reports whether the host can build this source.
func (s WidgetSource) IsValid() bool {
	for _, known := range WidgetSources() {
		if s == known {
			return true
		}
	}
	return false
}

// Widget size limits. A spec is author-controlled, so every unbounded dimension is a
// denial-of-service surface and gets a ceiling.
const (
	maxWidgetStats   = 12
	maxWidgetColumns = 16
	maxWidgetRows    = 250
	maxWidgetPoints  = 32
)

// WidgetSpec is the declarative description of one widget view.
type WidgetSpec struct {
	Type   WidgetType   `json:"type"`
	Source WidgetSource `json:"source"`

	// Filter is an expression evaluated per row; falsy rows are dropped. Optional.
	Filter string `json:"filter,omitempty"`

	// Stats populate a stat_grid. Their expressions are usually aggregates.
	Stats []WidgetStat `json:"stats,omitempty"`

	// Columns populate a table, one per output column.
	Columns []WidgetColumn `json:"columns,omitempty"`

	// Label and Value populate a chart: Label names a slice or bar, Value sizes it.
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
	// GroupBy collapses rows sharing a value into one point. When set, Value is
	// evaluated over each group (so it may be an aggregate); when unset, each row
	// becomes its own point.
	GroupBy string `json:"group_by,omitempty"`
	// Format applies to chart point values.
	Format WidgetFormat `json:"format,omitempty"`

	// SortBy is an expression for tables, or "value"/"label" for charts.
	SortBy string `json:"sort_by,omitempty"`
	// SortDesc reverses the sort.
	SortDesc bool `json:"sort_desc,omitempty"`
	// Limit caps output rows or points. Zero means the type's default ceiling.
	Limit int `json:"limit,omitempty"`

	// EmptyText is shown instead of an empty chart or table.
	EmptyText string `json:"empty_text,omitempty"`
}

// WidgetStat is one cell of a stat grid.
type WidgetStat struct {
	Label  string       `json:"label"`
	Value  string       `json:"value"`
	Format WidgetFormat `json:"format,omitempty"`
	Unit   string       `json:"unit,omitempty"`
	Help   string       `json:"help,omitempty"`
}

// WidgetColumn is one column of a table.
type WidgetColumn struct {
	Label  string       `json:"label"`
	Value  string       `json:"value"`
	Format WidgetFormat `json:"format,omitempty"`
	// Align is "left", "right", or "center"; it defaults by format.
	Align string `json:"align,omitempty"`
}

// WidgetFormat turns a raw value into a display string. Formatting happens server-side so
// that the same spec reads identically everywhere it is rendered.
type WidgetFormat string

const (
	FormatAuto     WidgetFormat = ""
	FormatNumber   WidgetFormat = "number"
	FormatInteger  WidgetFormat = "integer"
	FormatGrams    WidgetFormat = "grams"
	FormatKilos    WidgetFormat = "kilograms"
	FormatOunces   WidgetFormat = "ounces"
	FormatPounds   WidgetFormat = "pounds"
	FormatCents    WidgetFormat = "currency_cents"
	FormatPercent  WidgetFormat = "percent"
	FormatText     WidgetFormat = "text"
	FormatBool     WidgetFormat = "boolean"
	FormatDuration WidgetFormat = "duration_minutes"
)

var widgetFormats = map[WidgetFormat]bool{
	FormatAuto: true, FormatNumber: true, FormatInteger: true, FormatGrams: true,
	FormatKilos: true, FormatOunces: true, FormatPounds: true, FormatCents: true,
	FormatPercent: true, FormatText: true, FormatBool: true, FormatDuration: true,
}

// --- Render model ---

// WidgetRender is the fully evaluated widget. It contains no expressions, so the frontend
// is a dumb renderer and cannot be made to execute anything an author wrote.
type WidgetRender struct {
	Type      WidgetType `json:"type"`
	Empty     bool       `json:"empty"`
	EmptyText string     `json:"empty_text,omitempty"`

	Stats   []RenderedStat   `json:"stats,omitempty"`
	Columns []RenderedColumn `json:"columns,omitempty"`
	Rows    [][]RenderedCell `json:"rows,omitempty"`
	Points  []RenderedPoint  `json:"points,omitempty"`

	// Total is the sum of point values, which is what a pie chart needs to compute
	// slice angles without trusting the frontend to agree with us about rounding.
	Total float64 `json:"total,omitempty"`
	// Truncated reports that Limit cut results off, so the UI can say so.
	Truncated bool `json:"truncated,omitempty"`
}

// RenderedStat is one evaluated stat cell.
type RenderedStat struct {
	Label   string      `json:"label"`
	Value   interface{} `json:"value"`
	Display string      `json:"display"`
	Unit    string      `json:"unit,omitempty"`
	Help    string      `json:"help,omitempty"`
}

// RenderedColumn is a table column header.
type RenderedColumn struct {
	Label string `json:"label"`
	Align string `json:"align"`
}

// RenderedCell is one evaluated table cell.
type RenderedCell struct {
	Value   interface{} `json:"value"`
	Display string      `json:"display"`
}

// RenderedPoint is one bar or slice.
type RenderedPoint struct {
	Label   string  `json:"label"`
	Value   float64 `json:"value"`
	Display string  `json:"display"`
	// Share is the fraction of Total, precomputed for pie geometry.
	Share float64 `json:"share"`
}

// WidgetData is the evaluated input the host assembles for a widget.
type WidgetData struct {
	// Rows are the records the widget iterates, already resolved and redacted by the
	// caller. A widget never reaches into the database itself.
	Rows []map[string]interface{}
	// Context is merged underneath every row, so an expression can reference
	// loadout.stats.total_weight_g next to a per-row entry field.
	Context map[string]interface{}
}

// --- Validation ---

// ValidateWidgetSpec checks a spec at publish time, compiling every expression it
// contains. A plugin whose arithmetic does not parse cannot be published, which is what
// keeps render-time failures rare enough to be worth treating as bugs.
func ValidateWidgetSpec(spec WidgetSpec) error {
	if !spec.Source.IsValid() {
		return fmt.Errorf("unknown data source %q", spec.Source)
	}
	if spec.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if spec.Filter != "" {
		if _, err := ParseExpr(spec.Filter); err != nil {
			return fmt.Errorf("filter: %w", err)
		}
	}
	if !widgetFormats[spec.Format] {
		return fmt.Errorf("unknown format %q", spec.Format)
	}

	switch spec.Type {
	case WidgetStatGrid:
		if len(spec.Stats) == 0 {
			return fmt.Errorf("a stat_grid needs at least one stat")
		}
		if len(spec.Stats) > maxWidgetStats {
			return fmt.Errorf("a stat_grid allows at most %d stats, got %d", maxWidgetStats, len(spec.Stats))
		}
		for i, s := range spec.Stats {
			if strings.TrimSpace(s.Label) == "" {
				return fmt.Errorf("stat %d has no label", i)
			}
			if !widgetFormats[s.Format] {
				return fmt.Errorf("stat %q has unknown format %q", s.Label, s.Format)
			}
			if _, err := ParseExpr(s.Value); err != nil {
				return fmt.Errorf("stat %q: %w", s.Label, err)
			}
		}

	case WidgetTable:
		if len(spec.Columns) == 0 {
			return fmt.Errorf("a table needs at least one column")
		}
		if len(spec.Columns) > maxWidgetColumns {
			return fmt.Errorf("a table allows at most %d columns, got %d", maxWidgetColumns, len(spec.Columns))
		}
		for i, c := range spec.Columns {
			if strings.TrimSpace(c.Label) == "" {
				return fmt.Errorf("column %d has no label", i)
			}
			if !widgetFormats[c.Format] {
				return fmt.Errorf("column %q has unknown format %q", c.Label, c.Format)
			}
			if c.Align != "" && c.Align != "left" && c.Align != "right" && c.Align != "center" {
				return fmt.Errorf("column %q has unknown align %q", c.Label, c.Align)
			}
			if _, err := ParseExpr(c.Value); err != nil {
				return fmt.Errorf("column %q: %w", c.Label, err)
			}
		}
		if spec.SortBy != "" {
			if _, err := ParseExpr(spec.SortBy); err != nil {
				return fmt.Errorf("sort_by: %w", err)
			}
		}
		if spec.Limit > maxWidgetRows {
			return fmt.Errorf("a table allows at most %d rows, got limit %d", maxWidgetRows, spec.Limit)
		}

	case WidgetBarChart, WidgetPieChart:
		if strings.TrimSpace(spec.Label) == "" {
			return fmt.Errorf("a %s needs a label expression", spec.Type)
		}
		if strings.TrimSpace(spec.Value) == "" {
			return fmt.Errorf("a %s needs a value expression", spec.Type)
		}
		if _, err := ParseExpr(spec.Label); err != nil {
			return fmt.Errorf("label: %w", err)
		}
		if _, err := ParseExpr(spec.Value); err != nil {
			return fmt.Errorf("value: %w", err)
		}
		if spec.GroupBy != "" {
			gb, err := ParseExpr(spec.GroupBy)
			if err != nil {
				return fmt.Errorf("group_by: %w", err)
			}
			if UsesAggregate(gb) {
				return fmt.Errorf("group_by cannot aggregate; it names the group a row belongs to")
			}
		}
		if spec.SortBy != "" && spec.SortBy != "value" && spec.SortBy != "label" {
			return fmt.Errorf("a chart sorts by %q or %q, not %q", "value", "label", spec.SortBy)
		}
		if spec.Limit > maxWidgetPoints {
			return fmt.Errorf("a chart allows at most %d points, got limit %d", maxWidgetPoints, spec.Limit)
		}

	default:
		return fmt.Errorf("unknown widget type %q", spec.Type)
	}
	return nil
}

// --- Evaluation ---

// EvaluateWidget turns a spec plus data into a render model.
//
// Expressions were already compiled once at publish time; they are compiled again here
// because manifests are stored as JSON rather than as parsed trees. A compile failure at
// this point means something bypassed validation, so it is returned as an error rather
// than silently blanked.
func EvaluateWidget(spec WidgetSpec, data WidgetData) (WidgetRender, error) {
	out := WidgetRender{Type: spec.Type, EmptyText: spec.EmptyText}

	scopes := make([]map[string]interface{}, 0, len(data.Rows))
	for _, row := range data.Rows {
		scopes = append(scopes, mergeWidgetScope(data.Context, row))
	}

	if spec.Filter != "" {
		filter, err := ParseExpr(spec.Filter)
		if err != nil {
			return out, fmt.Errorf("filter: %w", err)
		}
		kept := scopes[:0:0]
		for _, s := range scopes {
			v, err := EvalExpr(filter, EvalScope{Row: s, Rows: scopes})
			if err != nil {
				return out, fmt.Errorf("filter: %w", err)
			}
			if Truthy(v) {
				kept = append(kept, s)
			}
		}
		scopes = kept
	}

	switch spec.Type {
	case WidgetStatGrid:
		return evaluateStatGrid(spec, data.Context, scopes, out)
	case WidgetTable:
		return evaluateTable(spec, scopes, out)
	case WidgetBarChart, WidgetPieChart:
		return evaluateChart(spec, scopes, out)
	}
	return out, fmt.Errorf("unknown widget type %q", spec.Type)
}

func evaluateStatGrid(spec WidgetSpec, context map[string]interface{}, scopes []map[string]interface{}, out WidgetRender) (WidgetRender, error) {
	// A stat grid has no per-row output, so every expression is evaluated once against
	// the context, with the row set available to aggregates.
	base := mergeWidgetScope(context, nil)
	for _, s := range spec.Stats {
		e, err := ParseExpr(s.Value)
		if err != nil {
			return out, fmt.Errorf("stat %q: %w", s.Label, err)
		}
		v, err := EvalExpr(e, EvalScope{Row: base, Rows: scopes})
		if err != nil {
			return out, fmt.Errorf("stat %q: %w", s.Label, err)
		}
		out.Stats = append(out.Stats, RenderedStat{
			Label:   s.Label,
			Value:   v,
			Display: FormatWidgetValue(v, s.Format),
			Unit:    s.Unit,
			Help:    s.Help,
		})
	}
	out.Empty = len(out.Stats) == 0
	return out, nil
}

func evaluateTable(spec WidgetSpec, scopes []map[string]interface{}, out WidgetRender) (WidgetRender, error) {
	for _, c := range spec.Columns {
		out.Columns = append(out.Columns, RenderedColumn{Label: c.Label, Align: alignFor(c)})
	}

	ordered, err := sortScopes(spec, scopes)
	if err != nil {
		return out, err
	}

	limit := spec.Limit
	if limit <= 0 || limit > maxWidgetRows {
		limit = maxWidgetRows
	}
	if len(ordered) > limit {
		ordered = ordered[:limit]
		out.Truncated = true
	}

	compiled := make([]Expr, len(spec.Columns))
	for i, c := range spec.Columns {
		e, err := ParseExpr(c.Value)
		if err != nil {
			return out, fmt.Errorf("column %q: %w", c.Label, err)
		}
		compiled[i] = e
	}

	for _, scope := range ordered {
		cells := make([]RenderedCell, 0, len(compiled))
		for i, e := range compiled {
			// Aggregates see the full filtered set, not the truncated page, so a
			// "percent of total" column stays honest when rows are cut off.
			v, err := EvalExpr(e, EvalScope{Row: scope, Rows: scopes})
			if err != nil {
				return out, fmt.Errorf("column %q: %w", spec.Columns[i].Label, err)
			}
			cells = append(cells, RenderedCell{Value: v, Display: FormatWidgetValue(v, spec.Columns[i].Format)})
		}
		out.Rows = append(out.Rows, cells)
	}
	out.Empty = len(out.Rows) == 0
	return out, nil
}

func evaluateChart(spec WidgetSpec, scopes []map[string]interface{}, out WidgetRender) (WidgetRender, error) {
	labelExpr, err := ParseExpr(spec.Label)
	if err != nil {
		return out, fmt.Errorf("label: %w", err)
	}
	valueExpr, err := ParseExpr(spec.Value)
	if err != nil {
		return out, fmt.Errorf("value: %w", err)
	}

	var points []RenderedPoint

	if spec.GroupBy == "" {
		// One point per row.
		for _, scope := range scopes {
			label, err := EvalExpr(labelExpr, EvalScope{Row: scope, Rows: scopes})
			if err != nil {
				return out, fmt.Errorf("label: %w", err)
			}
			value, err := EvalExpr(valueExpr, EvalScope{Row: scope, Rows: scopes})
			if err != nil {
				return out, fmt.Errorf("value: %w", err)
			}
			points = append(points, RenderedPoint{Label: widgetLabel(label), Value: numberOf(value)})
		}
	} else {
		groupExpr, err := ParseExpr(spec.GroupBy)
		if err != nil {
			return out, fmt.Errorf("group_by: %w", err)
		}
		// Partition, preserving first-seen order so the chart is stable between renders.
		var order []string
		groups := map[string][]map[string]interface{}{}
		for _, scope := range scopes {
			g, err := EvalExpr(groupExpr, EvalScope{Row: scope, Rows: scopes})
			if err != nil {
				return out, fmt.Errorf("group_by: %w", err)
			}
			key := widgetLabel(g)
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], scope)
		}

		aggregates := UsesAggregate(valueExpr)
		for _, key := range order {
			members := groups[key]
			label := key
			// A label expression distinct from the group key is evaluated against the
			// group's first member, which is how you get "Shelter" from a category id.
			if spec.Label != spec.GroupBy {
				l, err := EvalExpr(labelExpr, EvalScope{Row: members[0], Rows: members})
				if err != nil {
					return out, fmt.Errorf("label: %w", err)
				}
				label = widgetLabel(l)
			}

			var total float64
			if aggregates {
				// The expression folds the group itself, e.g. sum(weight * qty).
				v, err := EvalExpr(valueExpr, EvalScope{Row: members[0], Rows: members})
				if err != nil {
					return out, fmt.Errorf("value: %w", err)
				}
				total = numberOf(v)
			} else {
				// A plain per-row expression is summed, which is what "group by" means
				// to everyone who has ever written a GROUP BY.
				for _, m := range members {
					v, err := EvalExpr(valueExpr, EvalScope{Row: m, Rows: members})
					if err != nil {
						return out, fmt.Errorf("value: %w", err)
					}
					total += numberOf(v)
				}
			}
			points = append(points, RenderedPoint{Label: label, Value: total})
		}
	}

	sortPoints(spec, points)

	limit := spec.Limit
	if limit <= 0 || limit > maxWidgetPoints {
		limit = maxWidgetPoints
	}
	if len(points) > limit {
		points = points[:limit]
		out.Truncated = true
	}

	for _, p := range points {
		out.Total += p.Value
	}
	for i := range points {
		if out.Total != 0 {
			points[i].Share = points[i].Value / out.Total
		}
		points[i].Display = FormatWidgetValue(points[i].Value, spec.Format)
	}

	out.Points = points
	out.Empty = len(points) == 0
	return out, nil
}

// sortScopes orders table rows by an expression.
func sortScopes(spec WidgetSpec, scopes []map[string]interface{}) ([]map[string]interface{}, error) {
	ordered := make([]map[string]interface{}, len(scopes))
	copy(ordered, scopes)
	if spec.SortBy == "" {
		return ordered, nil
	}

	sortExpr, err := ParseExpr(spec.SortBy)
	if err != nil {
		return nil, fmt.Errorf("sort_by: %w", err)
	}
	// Evaluate the sort key once per row rather than inside the comparator, which keeps
	// an O(n log n) sort from doing O(n log n) expression evaluations.
	keys := make(map[int]interface{}, len(ordered))
	for i, s := range ordered {
		v, err := EvalExpr(sortExpr, EvalScope{Row: s, Rows: scopes})
		if err != nil {
			return nil, fmt.Errorf("sort_by: %w", err)
		}
		keys[i] = v
	}
	idx := make([]int, len(ordered))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		c := compare(keys[idx[a]], keys[idx[b]])
		if spec.SortDesc {
			return c > 0
		}
		return c < 0
	})
	sorted := make([]map[string]interface{}, len(ordered))
	for i, j := range idx {
		sorted[i] = ordered[j]
	}
	return sorted, nil
}

func sortPoints(spec WidgetSpec, points []RenderedPoint) {
	switch spec.SortBy {
	case "value":
		sort.SliceStable(points, func(a, b int) bool {
			if spec.SortDesc {
				return points[a].Value > points[b].Value
			}
			return points[a].Value < points[b].Value
		})
	case "label":
		sort.SliceStable(points, func(a, b int) bool {
			if spec.SortDesc {
				return points[a].Label > points[b].Label
			}
			return points[a].Label < points[b].Label
		})
	}
}

// mergeWidgetScope layers a row on top of the shared context. The copy is shallow and
// per-row, so an expression cannot mutate anything the next row will see.
func mergeWidgetScope(context, row map[string]interface{}) map[string]interface{} {
	scope := make(map[string]interface{}, len(context)+len(row))
	for k, v := range context {
		scope[k] = v
	}
	for k, v := range row {
		scope[k] = v
	}
	return scope
}

func alignFor(c WidgetColumn) string {
	if c.Align != "" {
		return c.Align
	}
	switch c.Format {
	case FormatText, FormatAuto, FormatBool:
		return "left"
	}
	return "right"
}

func widgetLabel(v interface{}) string {
	s := toString(v)
	if strings.TrimSpace(s) == "" {
		return "Unspecified"
	}
	return s
}

func numberOf(v interface{}) float64 {
	if v == nil {
		return 0
	}
	n := toNumber(v)
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return n
}

// FormatWidgetValue renders a value for display. Formatting lives on the server so that
// every surface showing the same widget agrees down to the last decimal place.
func FormatWidgetValue(v interface{}, f WidgetFormat) string {
	if v == nil {
		return "—"
	}
	switch f {
	case FormatText:
		return toString(v)
	case FormatBool:
		if Truthy(v) {
			return "Yes"
		}
		return "No"
	case FormatInteger:
		return withThousands(strconv.FormatInt(int64(math.Round(toNumber(v))), 10))
	case FormatGrams:
		return withThousands(strconv.FormatInt(int64(math.Round(toNumber(v))), 10)) + " g"
	case FormatKilos:
		return trimZeros(strconv.FormatFloat(toNumber(v)/1000, 'f', 2, 64)) + " kg"
	case FormatOunces:
		return trimZeros(strconv.FormatFloat(toNumber(v)/28.349523125, 'f', 1, 64)) + " oz"
	case FormatPounds:
		return trimZeros(strconv.FormatFloat(toNumber(v)/453.59237, 'f', 2, 64)) + " lb"
	case FormatCents:
		cents := math.Round(toNumber(v))
		return "$" + withThousands(strconv.FormatFloat(cents/100, 'f', 2, 64))
	case FormatPercent:
		return trimZeros(strconv.FormatFloat(toNumber(v), 'f', 1, 64)) + "%"
	case FormatDuration:
		return formatMinutes(toNumber(v))
	case FormatNumber:
		return withThousands(trimZeros(strconv.FormatFloat(toNumber(v), 'f', 2, 64)))
	}

	// FormatAuto: let the value decide, which keeps simple specs free of boilerplate.
	switch n := v.(type) {
	case string:
		return n
	case bool:
		if n {
			return "Yes"
		}
		return "No"
	}
	return withThousands(trimZeros(strconv.FormatFloat(toNumber(v), 'f', 2, 64)))
}

func formatMinutes(total float64) string {
	m := int64(math.Round(total))
	if m < 60 {
		return strconv.FormatInt(m, 10) + " min"
	}
	h := m / 60
	rem := m % 60
	if rem == 0 {
		return strconv.FormatInt(h, 10) + " h"
	}
	return strconv.FormatInt(h, 10) + " h " + strconv.FormatInt(rem, 10) + " min"
}

// trimZeros drops trailing decimal noise so "1.50" reads as "1.5" and "2.00" as "2".
func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// withThousands groups the integer part with commas.
func withThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	if len(intPart) > 3 {
		var sb strings.Builder
		lead := len(intPart) % 3
		if lead > 0 {
			sb.WriteString(intPart[:lead])
		}
		for i := lead; i < len(intPart); i += 3 {
			if sb.Len() > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(intPart[i : i+3])
		}
		intPart = sb.String()
	}
	if neg {
		return "-" + intPart + frac
	}
	return intPart + frac
}
