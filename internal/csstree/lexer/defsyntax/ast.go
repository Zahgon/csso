// Package defsyntax is a port of css-tree 2.2.0's definition-syntax module: it
// parses CSS Value Definition Syntax (the grammar notation used by specs and
// mdn-data, e.g. `<length> | auto`), serializes it back and walks it.
package defsyntax

// Node is a node of a parsed value definition. Kind returns the css-tree node
// type name ("Group", "Multiplier", "Type", "Property", "Keyword",
// "AtKeyword", "Function", "String", "Token" or "Comma").
type Node interface {
	Kind() string
}

// Group is a sequence of terms joined by one combinator: " " (juxtaposition),
// "&&", "||" or "|". Explicit groups were written in square brackets;
// DisallowEmpty marks the `[ ... ]!` form.
type Group struct {
	Terms         []Node
	Combinator    string
	DisallowEmpty bool
	Explicit      bool
}

// Multiplier repeats Term between Min and Max times; a Max of 0 means
// unbounded. Comma marks the comma-separated `#` multiplier.
type Multiplier struct {
	Comma bool
	Min   int
	Max   int
	Term  Node
}

// Type is a reference to a data type such as `<length>` or `<rgb()>`, with an
// optional numeric range restriction (`<number [0,∞]>`).
type Type struct {
	Name string
	Opts *Range
}

// Property is a reference to the value syntax of a property: `<'color'>`.
type Property struct {
	Name string
}

// Keyword is a literal identifier such as `auto`.
type Keyword struct {
	Name string
}

// AtKeyword is a literal at-keyword such as `@top-left`; Name excludes `@`.
type AtKeyword struct {
	Name string
}

// Function is the opening of a function, e.g. `rgb(`; Name excludes `(`. The
// closing parenthesis is a separate Token.
type Function struct {
	Name string
}

// String is a quoted literal; Value keeps the surrounding apostrophes, as
// css-tree does.
type String struct {
	Value string
}

// Token is any other single character literal, e.g. `/` or `)`.
type Token struct {
	Value string
}

// Comma is a literal comma.
type Comma struct{}

// Range is the bracketed range restriction of a Type. A nil bound stands for
// an infinite one (css-tree's null).
type Range struct {
	Min *RangeBound
	Max *RangeBound
}

// RangeBound is one end of a Range. css-tree reads a bound followed by a unit
// (`[0deg,360deg]`) as the string "0deg" and skips range validation for it;
// such bounds have a non-empty Unit.
type RangeBound struct {
	Value float64
	Unit  string
}

// Kind returns "Group".
func (*Group) Kind() string { return "Group" }

// Kind returns "Multiplier".
func (*Multiplier) Kind() string { return "Multiplier" }

// Kind returns "Type".
func (*Type) Kind() string { return "Type" }

// Kind returns "Property".
func (*Property) Kind() string { return "Property" }

// Kind returns "Keyword".
func (*Keyword) Kind() string { return "Keyword" }

// Kind returns "AtKeyword".
func (*AtKeyword) Kind() string { return "AtKeyword" }

// Kind returns "Function".
func (*Function) Kind() string { return "Function" }

// Kind returns "String".
func (*String) Kind() string { return "String" }

// Kind returns "Token".
func (*Token) Kind() string { return "Token" }

// Kind returns "Comma".
func (*Comma) Kind() string { return "Comma" }
