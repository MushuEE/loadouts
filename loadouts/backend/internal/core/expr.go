package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// A deliberately small expression language for the declarative widget tier.
//
// Widgets need computed values to be useful — "grams per dollar", "percent of base
// weight", "weight × quantity" — but plugin authors are untrusted, so this is explicitly
// NOT a scripting language. There is no assignment, no iteration, no user-defined
// functions, no recursion, and no way to reach the host. An expression is a pure function
// from a data row to a value.
//
// Grammar (precedence climbing, loosest to tightest):
//
//	|| && | == != | < <= > >= | + - | * / % | unary - ! | primary
//
// Primaries are numbers, strings, booleans, dotted field paths, parenthesised
// expressions, and calls to a fixed function set.
//
// Aggregate functions (sum, avg, min, max, count) are the one interesting piece: they
// re-evaluate their argument once per row of the current row set, so
// `sum(item.metadata.core.weight_g * entry.quantity)` works without any loop syntax.

// Expression limits. These exist so a hostile or careless manifest cannot turn a render
// into a denial of service.
const (
	maxExprLength = 1000
	maxExprDepth  = 32
)

// Expr is a parsed expression tree.
type Expr interface {
	node()
}

type exprNumber struct{ v float64 }
type exprString struct{ v string }
type exprBool struct{ v bool }
type exprNull struct{}
type exprPath struct{ parts []string }
type exprUnary struct {
	op string
	x  Expr
}
type exprBinary struct {
	op   string
	l, r Expr
}
type exprCall struct {
	name string
	args []Expr
}

func (exprNumber) node() {}
func (exprString) node() {}
func (exprBool) node()   {}
func (exprNull) node()   {}
func (exprPath) node()   {}
func (exprUnary) node()  {}
func (exprBinary) node() {}
func (exprCall) node()   {}

// --- Tokenizer ---

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokString
	tokIdent
	tokOp
	tokLParen
	tokRParen
	tokComma
)

type token struct {
	kind tokenKind
	text string
	num  float64
	pos  int
}

// multiCharOps must be checked before single-character operators.
var multiCharOps = []string{"==", "!=", "<=", ">=", "&&", "||"}

func tokenize(src string) ([]token, error) {
	var out []token
	i := 0
	for i < len(src) {
		c := src[i]

		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}

		if c == '(' {
			out = append(out, token{kind: tokLParen, text: "(", pos: i})
			i++
			continue
		}
		if c == ')' {
			out = append(out, token{kind: tokRParen, text: ")", pos: i})
			i++
			continue
		}
		if c == ',' {
			out = append(out, token{kind: tokComma, text: ",", pos: i})
			i++
			continue
		}

		// String literal, single or double quoted. No escape sequences: keeping the
		// lexer trivial removes a whole class of parsing surprises.
		if c == '\'' || c == '"' {
			quote := c
			j := i + 1
			for j < len(src) && src[j] != quote {
				j++
			}
			if j >= len(src) {
				return nil, fmt.Errorf("unterminated string starting at position %d", i)
			}
			out = append(out, token{kind: tokString, text: src[i+1 : j], pos: i})
			i = j + 1
			continue
		}

		if isDigit(c) || (c == '.' && i+1 < len(src) && isDigit(src[i+1])) {
			j := i
			for j < len(src) && (isDigit(src[j]) || src[j] == '.') {
				j++
			}
			f, err := strconv.ParseFloat(src[i:j], 64)
			if err != nil {
				return nil, fmt.Errorf("bad number %q at position %d", src[i:j], i)
			}
			out = append(out, token{kind: tokNumber, num: f, text: src[i:j], pos: i})
			i = j
			continue
		}

		// Identifiers and dotted paths.
		if isIdentStart(c) {
			j := i
			for j < len(src) && (isIdentChar(src[j]) || src[j] == '.') {
				j++
			}
			out = append(out, token{kind: tokIdent, text: src[i:j], pos: i})
			i = j
			continue
		}

		matched := false
		for _, op := range multiCharOps {
			if strings.HasPrefix(src[i:], op) {
				out = append(out, token{kind: tokOp, text: op, pos: i})
				i += len(op)
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		if strings.ContainsRune("+-*/%<>!", rune(c)) {
			out = append(out, token{kind: tokOp, text: string(c), pos: i})
			i++
			continue
		}

		return nil, fmt.Errorf("unexpected character %q at position %d", string(c), i)
	}

	out = append(out, token{kind: tokEOF, pos: len(src)})
	return out, nil
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentChar(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// --- Parser ---

type parser struct {
	tokens []token
	pos    int
	depth  int
}

// binaryPrecedence maps an operator to its binding power; higher binds tighter.
var binaryPrecedence = map[string]int{
	"||": 1,
	"&&": 2,
	"==": 3, "!=": 3,
	"<": 4, "<=": 4, ">": 4, ">=": 4,
	"+": 5, "-": 5,
	"*": 6, "/": 6, "%": 6,
}

// ParseExpr compiles an expression string into a tree, or explains why it cannot.
func ParseExpr(src string) (Expr, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("expression is empty")
	}
	if len(src) > maxExprLength {
		return nil, fmt.Errorf("expression is %d characters, limit is %d", len(src), maxExprLength)
	}

	tokens, err := tokenize(src)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	e, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("unexpected %q at position %d", p.peek().text, p.peek().pos)
	}
	return e, nil
}

func (p *parser) peek() token { return p.tokens[p.pos] }

func (p *parser) next() token {
	t := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *parser) parseBinary(minPrec int) (Expr, error) {
	p.depth++
	if p.depth > maxExprDepth {
		return nil, fmt.Errorf("expression nests deeper than %d levels", maxExprDepth)
	}
	defer func() { p.depth-- }()

	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t.kind != tokOp {
			break
		}
		prec, ok := binaryPrecedence[t.text]
		if !ok || prec < minPrec {
			break
		}
		p.next()
		// Left-associative: the right side binds at one level tighter.
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = exprBinary{op: t.text, l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	t := p.peek()
	if t.kind == tokOp && (t.text == "-" || t.text == "!") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return exprUnary{op: t.text, x: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.next()
	switch t.kind {
	case tokNumber:
		return exprNumber{v: t.num}, nil
	case tokString:
		return exprString{v: t.text}, nil
	case tokLParen:
		e, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, fmt.Errorf("missing closing parenthesis at position %d", p.peek().pos)
		}
		p.next()
		return e, nil
	case tokIdent:
		switch strings.ToLower(t.text) {
		case "true":
			return exprBool{v: true}, nil
		case "false":
			return exprBool{v: false}, nil
		case "null", "nil":
			return exprNull{}, nil
		}
		// A '(' immediately after an identifier makes it a call.
		if p.peek().kind == tokLParen {
			p.next()
			var args []Expr
			if p.peek().kind != tokRParen {
				for {
					a, err := p.parseBinary(0)
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.peek().kind == tokComma {
						p.next()
						continue
					}
					break
				}
			}
			if p.peek().kind != tokRParen {
				return nil, fmt.Errorf("missing closing parenthesis for %s() at position %d", t.text, p.peek().pos)
			}
			p.next()

			name := strings.ToLower(t.text)
			if !isKnownFunction(name) {
				return nil, fmt.Errorf("unknown function %q", t.text)
			}
			return exprCall{name: name, args: args}, nil
		}
		return exprPath{parts: strings.Split(t.text, ".")}, nil
	}
	return nil, fmt.Errorf("unexpected %q at position %d", t.text, t.pos)
}

// --- Evaluation ---

// EvalScope is the data an expression sees. Row is the current record; Rows is the set an
// aggregate function ranges over.
type EvalScope struct {
	Row  map[string]interface{}
	Rows []map[string]interface{}
}

var aggregateFunctions = map[string]bool{
	"sum": true, "avg": true, "min": true, "max": true, "count": true,
}

var scalarFunctions = map[string]bool{
	"if": true, "round": true, "abs": true, "coalesce": true,
	"concat": true, "lower": true, "upper": true, "len": true,
	"floor": true, "ceil": true, "percent": true,
}

func isKnownFunction(name string) bool {
	return aggregateFunctions[name] || scalarFunctions[name]
}

// EvalExpr evaluates a parsed expression against a scope.
func EvalExpr(e Expr, scope EvalScope) (interface{}, error) {
	return evalNode(e, scope, 0)
}

func evalNode(e Expr, scope EvalScope, depth int) (interface{}, error) {
	if depth > maxExprDepth {
		return nil, fmt.Errorf("expression too deeply nested")
	}
	switch n := e.(type) {
	case exprNumber:
		return n.v, nil
	case exprString:
		return n.v, nil
	case exprBool:
		return n.v, nil
	case exprNull:
		return nil, nil
	case exprPath:
		return resolvePath(n.parts, scope.Row), nil
	case exprUnary:
		return evalUnary(n, scope, depth)
	case exprBinary:
		return evalBinary(n, scope, depth)
	case exprCall:
		return evalCall(n, scope, depth)
	}
	return nil, fmt.Errorf("unsupported expression node")
}

func evalUnary(n exprUnary, scope EvalScope, depth int) (interface{}, error) {
	x, err := evalNode(n.x, scope, depth+1)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "-":
		return -toNumber(x), nil
	case "!":
		return !Truthy(x), nil
	}
	return nil, fmt.Errorf("unsupported unary operator %q", n.op)
}

func evalBinary(n exprBinary, scope EvalScope, depth int) (interface{}, error) {
	// Short-circuit before evaluating the right side.
	if n.op == "&&" || n.op == "||" {
		l, err := evalNode(n.l, scope, depth+1)
		if err != nil {
			return nil, err
		}
		if n.op == "&&" && !Truthy(l) {
			return false, nil
		}
		if n.op == "||" && Truthy(l) {
			return true, nil
		}
		r, err := evalNode(n.r, scope, depth+1)
		if err != nil {
			return nil, err
		}
		return Truthy(r), nil
	}

	l, err := evalNode(n.l, scope, depth+1)
	if err != nil {
		return nil, err
	}
	r, err := evalNode(n.r, scope, depth+1)
	if err != nil {
		return nil, err
	}

	switch n.op {
	case "+":
		// String concatenation when either side is a string, arithmetic otherwise.
		if ls, ok := l.(string); ok {
			return ls + toString(r), nil
		}
		if rs, ok := r.(string); ok {
			return toString(l) + rs, nil
		}
		return toNumber(l) + toNumber(r), nil
	case "-":
		return toNumber(l) - toNumber(r), nil
	case "*":
		return toNumber(l) * toNumber(r), nil
	case "/":
		d := toNumber(r)
		if d == 0 {
			// Division by zero yields null rather than an error: a widget dividing by an
			// empty column should render a blank cell, not fail the whole page.
			return nil, nil
		}
		return toNumber(l) / d, nil
	case "%":
		d := toNumber(r)
		if d == 0 {
			return nil, nil
		}
		return math.Mod(toNumber(l), d), nil
	case "==":
		return looseEqual(l, r), nil
	case "!=":
		return !looseEqual(l, r), nil
	case "<":
		return compare(l, r) < 0, nil
	case "<=":
		return compare(l, r) <= 0, nil
	case ">":
		return compare(l, r) > 0, nil
	case ">=":
		return compare(l, r) >= 0, nil
	}
	return nil, fmt.Errorf("unsupported operator %q", n.op)
}

func evalCall(n exprCall, scope EvalScope, depth int) (interface{}, error) {
	if aggregateFunctions[n.name] {
		return evalAggregate(n, scope, depth)
	}

	args := make([]interface{}, 0, len(n.args))
	// `if` is lazy in its branches so that if(qty > 0, total/qty, 0) is safe.
	if n.name == "if" {
		if len(n.args) != 3 {
			return nil, fmt.Errorf("if() takes 3 arguments, got %d", len(n.args))
		}
		cond, err := evalNode(n.args[0], scope, depth+1)
		if err != nil {
			return nil, err
		}
		if Truthy(cond) {
			return evalNode(n.args[1], scope, depth+1)
		}
		return evalNode(n.args[2], scope, depth+1)
	}

	for _, a := range n.args {
		v, err := evalNode(a, scope, depth+1)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}

	switch n.name {
	case "round":
		places := 0.0
		if len(args) > 1 {
			places = toNumber(args[1])
		}
		mult := math.Pow(10, places)
		return math.Round(toNumber(args[0])*mult) / mult, nil
	case "floor":
		return math.Floor(toNumber(first(args))), nil
	case "ceil":
		return math.Ceil(toNumber(first(args))), nil
	case "abs":
		return math.Abs(toNumber(first(args))), nil
	case "percent":
		if len(args) != 2 {
			return nil, fmt.Errorf("percent() takes 2 arguments, got %d", len(args))
		}
		d := toNumber(args[1])
		if d == 0 {
			return nil, nil
		}
		return toNumber(args[0]) / d * 100, nil
	case "coalesce":
		for _, a := range args {
			if a != nil && a != "" {
				return a, nil
			}
		}
		return nil, nil
	case "concat":
		var sb strings.Builder
		for _, a := range args {
			sb.WriteString(toString(a))
		}
		return sb.String(), nil
	case "lower":
		return strings.ToLower(toString(first(args))), nil
	case "upper":
		return strings.ToUpper(toString(first(args))), nil
	case "len":
		return float64(len(toString(first(args)))), nil
	}
	return nil, fmt.Errorf("unknown function %q", n.name)
}

// evalAggregate re-evaluates its argument once per row. This is what lets the language
// express `sum(item.metadata.core.weight_g * entry.quantity)` without loop syntax.
func evalAggregate(n exprCall, scope EvalScope, depth int) (interface{}, error) {
	if len(n.args) != 1 {
		return nil, fmt.Errorf("%s() takes exactly 1 argument, got %d", n.name, len(n.args))
	}
	if containsAggregate(n.args[0]) {
		return nil, fmt.Errorf("%s() cannot contain another aggregate function", n.name)
	}

	if n.name == "count" {
		// count() counts rows where the argument is truthy, so count(item.verified)
		// reads naturally. count(1) counts every row.
		total := 0.0
		for _, row := range scope.Rows {
			v, err := evalNode(n.args[0], EvalScope{Row: row, Rows: scope.Rows}, depth+1)
			if err != nil {
				return nil, err
			}
			if Truthy(v) {
				total++
			}
		}
		return total, nil
	}

	var values []float64
	for _, row := range scope.Rows {
		v, err := evalNode(n.args[0], EvalScope{Row: row, Rows: scope.Rows}, depth+1)
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue // Skip missing values rather than treating them as zero.
		}
		values = append(values, toNumber(v))
	}
	if len(values) == 0 {
		if n.name == "sum" {
			return 0.0, nil
		}
		return nil, nil
	}

	switch n.name {
	case "sum":
		total := 0.0
		for _, v := range values {
			total += v
		}
		return total, nil
	case "avg":
		total := 0.0
		for _, v := range values {
			total += v
		}
		return total / float64(len(values)), nil
	case "min":
		m := values[0]
		for _, v := range values {
			if v < m {
				m = v
			}
		}
		return m, nil
	case "max":
		m := values[0]
		for _, v := range values {
			if v > m {
				m = v
			}
		}
		return m, nil
	}
	return nil, fmt.Errorf("unknown aggregate %q", n.name)
}

func containsAggregate(e Expr) bool {
	switch n := e.(type) {
	case exprCall:
		if aggregateFunctions[n.name] {
			return true
		}
		for _, a := range n.args {
			if containsAggregate(a) {
				return true
			}
		}
	case exprBinary:
		return containsAggregate(n.l) || containsAggregate(n.r)
	case exprUnary:
		return containsAggregate(n.x)
	}
	return false
}

// UsesAggregate reports whether an expression aggregates over rows, which tells a widget
// whether to evaluate it once or per row.
func UsesAggregate(e Expr) bool { return containsAggregate(e) }

// --- Value helpers ---

func first(args []interface{}) interface{} {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

// resolvePath walks a dotted path through nested maps. A missing key yields nil rather
// than an error: widgets routinely reference metadata that only some items carry.
func resolvePath(parts []string, row map[string]interface{}) interface{} {
	var current interface{} = row
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// toNumber coerces a value to float64, yielding 0 for anything non-numeric.
func toNumber(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

func toString(v interface{}) string {
	switch n := v.(type) {
	case nil:
		return ""
	case string:
		return n
	case float64:
		if n == math.Trunc(n) && math.Abs(n) < 1e15 {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(n)
	}
	return fmt.Sprintf("%v", v)
}

// Truthy defines truthiness: nil, false, 0, and "" are false; everything else is true.
func Truthy(v interface{}) bool {
	switch n := v.(type) {
	case nil:
		return false
	case bool:
		return n
	case string:
		return n != ""
	case float64:
		return n != 0
	}
	return toNumber(v) != 0
}

func looseEqual(l, r interface{}) bool {
	if l == nil || r == nil {
		return l == nil && r == nil
	}
	ls, lIsStr := l.(string)
	rs, rIsStr := r.(string)
	if lIsStr && rIsStr {
		return ls == rs
	}
	if lb, ok := l.(bool); ok {
		return lb == Truthy(r)
	}
	if rb, ok := r.(bool); ok {
		return rb == Truthy(l)
	}
	if lIsStr != rIsStr {
		return toString(l) == toString(r)
	}
	return toNumber(l) == toNumber(r)
}

// compare orders two values, comparing strings lexically and everything else numerically.
func compare(l, r interface{}) int {
	ls, lIsStr := l.(string)
	rs, rIsStr := r.(string)
	if lIsStr && rIsStr {
		return strings.Compare(ls, rs)
	}
	ln, rn := toNumber(l), toNumber(r)
	switch {
	case ln < rn:
		return -1
	case ln > rn:
		return 1
	}
	return 0
}
