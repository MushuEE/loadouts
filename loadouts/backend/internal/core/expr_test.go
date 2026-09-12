package core

import (
	"math"
	"strings"
	"testing"
)

// evalString is the shorthand the table tests use: parse, evaluate, return.
func evalString(t *testing.T, src string, scope EvalScope) interface{} {
	t.Helper()
	e, err := ParseExpr(src)
	if err != nil {
		t.Fatalf("ParseExpr(%q) failed: %v", src, err)
	}
	v, err := EvalExpr(e, scope)
	if err != nil {
		t.Fatalf("EvalExpr(%q) failed: %v", src, err)
	}
	return v
}

func TestParseExprRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"empty", "", "empty"},
		{"whitespace only", "   ", "empty"},
		{"too long", strings.Repeat("1+", 600) + "1", "limit is"},
		{"unterminated string", "'hello", "unterminated string"},
		{"unknown function", "frobnicate(1)", "unknown function"},
		{"trailing operator", "1 +", "unexpected"},
		{"missing paren", "(1 + 2", "missing closing parenthesis"},
		{"illegal character", "1 @ 2", "unexpected character"},
		{"too deep", strings.Repeat("(", 40) + "1" + strings.Repeat(")", 40), "nests deeper"},
		{"bare operator", "*", "unexpected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseExpr(tc.src)
			if err == nil {
				t.Fatalf("ParseExpr(%q) succeeded, want an error", tc.src)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseExpr(%q) error = %q, want it to mention %q", tc.src, err, tc.want)
			}
		})
	}
}

func TestEvalArithmeticAndPrecedence(t *testing.T) {
	cases := []struct {
		src  string
		want float64
	}{
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"10 - 2 - 3", 5},    // left-associative
		{"100 / 5 / 2", 10},  // left-associative
		{"2 + 3 * 4 - 6", 8}, // mixed
		{"-4 + 10", 6},
		{"-(2 + 3)", -5},
		{"7 % 3", 1},
		{"1.5 * 4", 6},
		{".5 + .25", 0.75},
		{"round(3.14159, 2)", 3.14},
		{"round(2.5)", 3},
		{"floor(3.9)", 3},
		{"ceil(3.1)", 4},
		{"abs(0 - 12)", 12},
		{"percent(25, 200)", 12.5},
		{"len('abcd')", 4},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got := evalString(t, tc.src, EvalScope{})
			if n, ok := got.(float64); !ok || math.Abs(n-tc.want) > 1e-9 {
				t.Fatalf("%s = %#v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestEvalDivisionByZeroIsNull(t *testing.T) {
	// A widget dividing by an empty column should render a blank cell, not fail the
	// entire page, so division by zero is null rather than an error.
	for _, src := range []string{"1 / 0", "1 % 0", "percent(5, 0)", "10 / (2 - 2)"} {
		if got := evalString(t, src, EvalScope{}); got != nil {
			t.Fatalf("%s = %#v, want nil", src, got)
		}
	}
}

func TestEvalPathsResolveThroughNestedMaps(t *testing.T) {
	scope := EvalScope{Row: map[string]interface{}{
		"entry": map[string]interface{}{"quantity": 2.0},
		"item": map[string]interface{}{
			"name": "Tarp",
			"metadata": map[string]interface{}{
				"core": map[string]interface{}{"weight_g": 240.0},
			},
		},
	}}

	if got := evalString(t, "item.metadata.core.weight_g * entry.quantity", scope); got != 480.0 {
		t.Fatalf("weight * quantity = %#v, want 480", got)
	}
	if got := evalString(t, "item.name", scope); got != "Tarp" {
		t.Fatalf("item.name = %#v, want Tarp", got)
	}

	// Missing keys yield nil rather than an error: widgets routinely reference metadata
	// only some items carry.
	for _, src := range []string{"item.metadata.ul.score", "nope", "item.name.deeper", "a.b.c.d"} {
		if got := evalString(t, src, scope); got != nil {
			t.Fatalf("%s = %#v, want nil for a missing path", src, got)
		}
	}
}

func TestEvalComparisonsAndEquality(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"1 < 2", true},
		{"2 <= 2", true},
		{"3 > 4", false},
		{"4 >= 4", true},
		{"1 == 1", true},
		{"1 != 2", true},
		{"'a' < 'b'", true},
		{"'abc' == 'abc'", true},
		{"'abc' == 'abd'", false},
		{"true == true", true},
		{"null == null", true},
		{"null == 0", false}, // null is its own thing, not a zero
		{"'2' == 2", true},   // loose equality across a string/number boundary
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := evalString(t, tc.src, EvalScope{}); got != tc.want {
				t.Fatalf("%s = %#v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestEvalLogicalOperatorsShortCircuit(t *testing.T) {
	// The right side of a short-circuited operator is never evaluated, which is what
	// makes `qty > 0 && total / qty > 5` safe to write.
	cases := []struct {
		src  string
		want bool
	}{
		{"true && true", true},
		{"true && false", false},
		{"false && nope.deeper", false},
		{"true || nope.deeper", true},
		{"false || true", true},
		{"!false", true},
		{"!0", true},
		{"!''", true},
		{"!'text'", false},
		{"1 > 0 && 2 > 1", true},
		{"1 > 0 || 2 > 3", true},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := evalString(t, tc.src, EvalScope{}); got != tc.want {
				t.Fatalf("%s = %#v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestEvalStringConcatenation(t *testing.T) {
	// `+` concatenates when either operand is a string and adds otherwise.
	cases := []struct {
		src  string
		want interface{}
	}{
		{"'a' + 'b'", "ab"},
		{"'weight: ' + 240", "weight: 240"},
		{"240 + ' g'", "240 g"},
		{"concat('a', 1, true)", "a1true"},
		{"upper('abc')", "ABC"},
		{"lower('ABC')", "abc"},
		{"2 + 2", 4.0},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			if got := evalString(t, tc.src, EvalScope{}); got != tc.want {
				t.Fatalf("%s = %#v, want %#v", tc.src, got, tc.want)
			}
		})
	}
}

func TestEvalIfIsLazy(t *testing.T) {
	// Only the taken branch is evaluated, so the untaken one may be nonsense.
	scope := EvalScope{Row: map[string]interface{}{"qty": 0.0}}
	if got := evalString(t, "if(qty > 0, 100 / qty, 0)", scope); got != 0.0 {
		t.Fatalf("if with a false condition = %#v, want 0", got)
	}

	scope = EvalScope{Row: map[string]interface{}{"qty": 4.0}}
	if got := evalString(t, "if(qty > 0, 100 / qty, 0)", scope); got != 25.0 {
		t.Fatalf("if with a true condition = %#v, want 25", got)
	}

	if got := evalString(t, "if(true, 'yes', 'no')", EvalScope{}); got != "yes" {
		t.Fatalf("if = %#v, want yes", got)
	}

	e, err := ParseExpr("if(1, 2)")
	if err != nil {
		t.Fatalf("ParseExpr failed: %v", err)
	}
	if _, err := EvalExpr(e, EvalScope{}); err == nil {
		t.Fatal("if() with 2 arguments succeeded, want an arity error")
	}
}

func TestEvalCoalesce(t *testing.T) {
	scope := EvalScope{Row: map[string]interface{}{"present": "here"}}
	if got := evalString(t, "coalesce(missing, present, 'fallback')", scope); got != "here" {
		t.Fatalf("coalesce = %#v, want here", got)
	}
	if got := evalString(t, "coalesce(missing, alsomissing, 'fallback')", scope); got != "fallback" {
		t.Fatalf("coalesce = %#v, want fallback", got)
	}
	if got := evalString(t, "coalesce(missing)", scope); got != nil {
		t.Fatalf("coalesce of nothing = %#v, want nil", got)
	}
}

// rows models a small loadout for the aggregate tests.
func aggregateRows() []map[string]interface{} {
	return []map[string]interface{}{
		{"w": 500.0, "q": 1.0, "verified": true, "cat": "shelter"},
		{"w": 250.0, "q": 2.0, "verified": false, "cat": "sleep"},
		{"w": 100.0, "q": 3.0, "verified": true, "cat": "sleep"},
	}
}

func TestEvalAggregates(t *testing.T) {
	rows := aggregateRows()
	scope := EvalScope{Row: map[string]interface{}{}, Rows: rows}

	cases := []struct {
		src  string
		want float64
	}{
		{"sum(w)", 850},
		{"sum(w * q)", 1300}, // the argument is re-evaluated per row
		{"avg(w)", 850.0 / 3},
		{"min(w)", 100},
		{"max(w)", 500},
		{"count(1)", 3},
		{"count(verified)", 2}, // count() counts truthy rows
		{"count(w > 200)", 2},  // ...including a computed predicate
		{"sum(w) / count(1)", 850.0 / 3},
		{"percent(sum(w * q), sum(w))", 1300.0 / 850.0 * 100},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got := evalString(t, tc.src, scope)
			n, ok := got.(float64)
			if !ok || math.Abs(n-tc.want) > 1e-9 {
				t.Fatalf("%s = %#v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestEvalAggregatesOverEmptyRowSet(t *testing.T) {
	empty := EvalScope{Rows: nil}
	// An empty sum is zero because adding nothing is a well-defined operation; an empty
	// min or average is null because there is no honest answer.
	if got := evalString(t, "sum(w)", empty); got != 0.0 {
		t.Fatalf("sum over no rows = %#v, want 0", got)
	}
	if got := evalString(t, "count(1)", empty); got != 0.0 {
		t.Fatalf("count over no rows = %#v, want 0", got)
	}
	for _, src := range []string{"avg(w)", "min(w)", "max(w)"} {
		if got := evalString(t, src, empty); got != nil {
			t.Fatalf("%s over no rows = %#v, want nil", src, got)
		}
	}
}

func TestEvalAggregateSkipsMissingValues(t *testing.T) {
	// A row missing the field is skipped rather than counted as zero, so an average is
	// not silently dragged down by items that never had the metadata.
	rows := []map[string]interface{}{
		{"score": 8.0},
		{},
		{"score": 6.0},
	}
	scope := EvalScope{Rows: rows}
	if got := evalString(t, "avg(score)", scope); got != 7.0 {
		t.Fatalf("avg(score) = %#v, want 7", got)
	}
	if got := evalString(t, "count(score)", scope); got != 2.0 {
		t.Fatalf("count(score) = %#v, want 2", got)
	}
}

func TestEvalRejectsNestedAggregates(t *testing.T) {
	e, err := ParseExpr("sum(sum(w))")
	if err != nil {
		t.Fatalf("ParseExpr failed: %v", err)
	}
	if _, err := EvalExpr(e, EvalScope{Rows: aggregateRows()}); err == nil {
		t.Fatal("nested aggregate evaluated, want an error")
	}

	e, err = ParseExpr("sum(w * avg(w))")
	if err != nil {
		t.Fatalf("ParseExpr failed: %v", err)
	}
	if _, err := EvalExpr(e, EvalScope{Rows: aggregateRows()}); err == nil {
		t.Fatal("aggregate nested inside arithmetic evaluated, want an error")
	}
}

func TestEvalAggregateArity(t *testing.T) {
	for _, src := range []string{"sum()", "sum(1, 2)"} {
		e, err := ParseExpr(src)
		if err != nil {
			t.Fatalf("ParseExpr(%q) failed: %v", src, err)
		}
		if _, err := EvalExpr(e, EvalScope{}); err == nil {
			t.Fatalf("%s evaluated, want an arity error", src)
		}
	}
}

func TestUsesAggregate(t *testing.T) {
	cases := map[string]bool{
		"w * q":                false,
		"sum(w)":               true,
		"w / sum(w)":           true,
		"if(w > 0, 1, 0)":      false,
		"if(w > 0, sum(w), 0)": true,
		"-count(1)":            true,
		"'text'":               false,
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			e, err := ParseExpr(src)
			if err != nil {
				t.Fatalf("ParseExpr failed: %v", err)
			}
			if got := UsesAggregate(e); got != want {
				t.Fatalf("UsesAggregate(%q) = %v, want %v", src, got, want)
			}
		})
	}
}

func TestTruthy(t *testing.T) {
	cases := []struct {
		v    interface{}
		want bool
	}{
		{nil, false},
		{false, false},
		{true, true},
		{0.0, false},
		{1.0, true},
		{-1.0, true},
		{"", false},
		{"x", true},
		{"0", true}, // a non-empty string is truthy even if it reads as zero
		{0, false},
		{3, true},
	}
	for _, tc := range cases {
		if got := Truthy(tc.v); got != tc.want {
			t.Fatalf("Truthy(%#v) = %v, want %v", tc.v, got, tc.want)
		}
	}
}
