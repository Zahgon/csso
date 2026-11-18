// Package csstree is a Go port of css-tree 2.2.0 (tokenizer, parser, walker,
// generator and utilities) configured the way csso 5.0.5 forks it.
package csstree

// Position is a point in the source text. Offset is a zero-based index into
// the source measured in UTF-16 code units (as JavaScript strings are), Line
// and Column are one-based.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Location is the source range a node was parsed from, matching css-tree's
// `loc` object ({ source, start, end }).
type Location struct {
	Source string
	Start  Position
	End    Position
}

// Node is implemented by every AST node. Nodes are always pointers, so
// passes can mutate them in place.
type Node interface {
	// Type returns the css-tree node type name, e.g. "Declaration".
	Type() string
	// Loc returns the node location, or nil when the parser was not asked
	// for positions or the node was synthesized.
	Loc() *Location
	// SetLoc replaces the node location.
	SetLoc(loc *Location)
}

// Base carries the location shared by every node type. It is embedded in
// each node struct.
type Base struct {
	Location *Location
}

// Loc returns the node location (nil when absent).
func (b *Base) Loc() *Location { return b.Location }

// SetLoc replaces the node location.
func (b *Base) SetLoc(loc *Location) { b.Location = loc }

// StyleSheet is the root node of a parsed stylesheet.
type StyleSheet struct {
	Base
	Children *List // Comment, CDO, CDC, Atrule, Rule, Raw

	// ID is an annotation csso's compressor assigns to a stylesheet (and
	// atrule blocks) to group rules during restructuring.
	ID int
	// FirstAtrulesAllowed is an annotation used by csso's cleaner to track
	// whether @charset/@import may still appear.
	FirstAtrulesAllowed bool
}

// Atrule is an at-rule such as @media or @import.
type Atrule struct {
	Base
	Name    string
	Prelude Node   // *AtrulePrelude, *Raw or nil
	Block   *Block // nil when the at-rule ends with a semicolon
}

// AtrulePrelude holds the parsed prelude of an at-rule.
type AtrulePrelude struct {
	Base
	Children *List

	// ID is csso's annotation holding the generated prelude text.
	ID string
}

// Rule is a qualified rule: a selector prelude followed by a block.
type Rule struct {
	Base
	Prelude Node // *SelectorList or *Raw
	Block   *Block

	// PseudoSignature is csso's annotation listing the "freezing" pseudo
	// classes/elements of the selector ("" stands for JavaScript false).
	PseudoSignature string
}

// SelectorList is a comma separated list of selectors.
type SelectorList struct {
	Base
	Children *List // Selector, Raw
}

// Selector is a complex selector.
type Selector struct {
	Base
	Children *List // TypeSelector, IdSelector, ClassSelector, AttributeSelector, PseudoClassSelector, PseudoElementSelector, Combinator, WhiteSpace

	// ID is csso's annotation holding the generated selector text.
	ID string
	// CompareMarker is csso's annotation describing the selector specificity
	// and scope used to decide whether rules may be reordered.
	CompareMarker string
}

// Block is a curly-bracket block of rules or declarations.
type Block struct {
	Base
	Children *List // Atrule, Rule, Declaration

	// ID is csso's annotation numbering at-rule blocks (0 when unset).
	ID int
	// AvoidRulesMerge is csso's annotation set on @keyframes blocks.
	AvoidRulesMerge bool
}

// Declaration is a property/value pair.
type Declaration struct {
	Base
	// Important is "" when the declaration has no `!` suffix, "important"
	// for `!important` (css-tree's `true`), and otherwise the identifier that
	// followed `!` verbatim (css-tree keeps e.g. `!ie` or `!IMPORTANT` as a
	// string).
	Important string
	Property  string
	Value     Node // *Value or *Raw

	// ID is csso's annotation identifying equal declarations (0 when unset).
	ID int
	// Length is csso's annotation holding the generated declaration length.
	Length int
	// Fingerprint is csso's annotation used while restructuring blocks.
	Fingerprint string
}

// DeclarationList is the result of parsing in the declarationList context.
type DeclarationList struct {
	Base
	Children *List // Declaration
}

// Value is a declaration value.
type Value struct {
	Base
	Children *List
}

// Identifier is an identifier.
type Identifier struct {
	Base
	Name string
}

// Number is a numeric literal kept as its source text.
type Number struct {
	Base
	Value string
}

// Dimension is a number followed by a unit.
type Dimension struct {
	Base
	Value string
	Unit  string
}

// Percentage is a number followed by `%`; Value excludes the sign.
type Percentage struct {
	Base
	Value string
}

// String is a quoted string; Value is the decoded content.
type String struct {
	Base
	Value string
}

// Url is a url() reference; Value is the decoded URL.
type Url struct {
	Base
	Value string
}

// Hash is a hash token in a value (e.g. a hex colour); Value excludes `#`.
type Hash struct {
	Base
	Value string
}

// Function is a function call with its arguments.
type Function struct {
	Base
	Name     string
	Children *List
}

// Operator is a single-character operator such as `/`, `,` or `+`.
type Operator struct {
	Base
	Value string
}

// Parentheses is a parenthesized sequence.
type Parentheses struct {
	Base
	Children *List
}

// Brackets is a square-bracketed sequence.
type Brackets struct {
	Base
	Children *List
}

// Raw is unparsed source text, produced when a part of the input is not
// parsed (by option or after a parse error).
type Raw struct {
	Base
	Value string

	// ID is csso's annotation holding the generated text when a Raw node
	// stands in for an at-rule prelude.
	ID string
}

// Comment is a comment; Value is the text between `/*` and `*/`.
type Comment struct {
	Base
	Value string
}

// WhiteSpace is significant whitespace; the parser always stores " ".
type WhiteSpace struct {
	Base
	Value string
}

// CDO is the `<!--` token.
type CDO struct {
	Base
}

// CDC is the `-->` token.
type CDC struct {
	Base
}

// TypeSelector is a type (or universal) selector, possibly namespaced.
type TypeSelector struct {
	Base
	Name string
}

// IdSelector is an `#id` selector; Name excludes `#`.
type IdSelector struct {
	Base
	Name string
}

// ClassSelector is a `.class` selector; Name excludes `.`.
type ClassSelector struct {
	Base
	Name string
}

// AttributeSelector is an `[attr]` selector.
type AttributeSelector struct {
	Base
	Name    *Identifier
	Matcher string // "" when absent
	Value   Node   // *String, *Identifier or nil
	Flags   string // "" when absent
}

// PseudoClassSelector is a `:name` or `:name(...)` selector.
type PseudoClassSelector struct {
	Base
	Name     string
	Children *List // nil for the non-functional form
}

// PseudoElementSelector is a `::name` or `::name(...)` selector.
type PseudoElementSelector struct {
	Base
	Name     string
	Children *List // nil for the non-functional form
}

// Combinator is a selector combinator: " ", ">", "+", "~" or "/deep/".
type Combinator struct {
	Base
	Name string
}

// Nth is the argument of :nth-child() and friends.
type Nth struct {
	Base
	Nth      Node          // *AnPlusB or *Identifier
	Selector *SelectorList // the `of <selector>` part, or nil
}

// AnPlusB is an An+B microsyntax value. Empty strings stand for css-tree's
// null coefficients.
type AnPlusB struct {
	Base
	A string
	B string
}

// MediaQueryList is a comma separated list of media queries.
type MediaQueryList struct {
	Base
	Children *List // MediaQuery
}

// MediaQuery is a single media query.
type MediaQuery struct {
	Base
	Children *List // Identifier, MediaFeature, WhiteSpace
}

// MediaFeature is a parenthesized media feature, e.g. `(min-width: 1px)`.
type MediaFeature struct {
	Base
	Name  string
	Value Node // *Identifier, *Number, *Dimension, *Ratio or nil
}

// Ratio is a `<number> / <number>` media feature value.
type Ratio struct {
	Base
	Left  string
	Right string
}

// UnicodeRange is a unicode-range value such as `U+0025-00FF`.
type UnicodeRange struct {
	Base
	Value string
}

// Type returns "StyleSheet".
func (*StyleSheet) Type() string { return "StyleSheet" }

// Type returns "Atrule".
func (*Atrule) Type() string { return "Atrule" }

// Type returns "AtrulePrelude".
func (*AtrulePrelude) Type() string { return "AtrulePrelude" }

// Type returns "Rule".
func (*Rule) Type() string { return "Rule" }

// Type returns "SelectorList".
func (*SelectorList) Type() string { return "SelectorList" }

// Type returns "Selector".
func (*Selector) Type() string { return "Selector" }

// Type returns "Block".
func (*Block) Type() string { return "Block" }

// Type returns "Declaration".
func (*Declaration) Type() string { return "Declaration" }

// Type returns "DeclarationList".
func (*DeclarationList) Type() string { return "DeclarationList" }

// Type returns "Value".
func (*Value) Type() string { return "Value" }

// Type returns "Identifier".
func (*Identifier) Type() string { return "Identifier" }

// Type returns "Number".
func (*Number) Type() string { return "Number" }

// Type returns "Dimension".
func (*Dimension) Type() string { return "Dimension" }

// Type returns "Percentage".
func (*Percentage) Type() string { return "Percentage" }

// Type returns "String".
func (*String) Type() string { return "String" }

// Type returns "Url".
func (*Url) Type() string { return "Url" }

// Type returns "Hash".
func (*Hash) Type() string { return "Hash" }

// Type returns "Function".
func (*Function) Type() string { return "Function" }

// Type returns "Operator".
func (*Operator) Type() string { return "Operator" }

// Type returns "Parentheses".
func (*Parentheses) Type() string { return "Parentheses" }

// Type returns "Brackets".
func (*Brackets) Type() string { return "Brackets" }

// Type returns "Raw".
func (*Raw) Type() string { return "Raw" }

// Type returns "Comment".
func (*Comment) Type() string { return "Comment" }

// Type returns "WhiteSpace".
func (*WhiteSpace) Type() string { return "WhiteSpace" }

// Type returns "CDO".
func (*CDO) Type() string { return "CDO" }

// Type returns "CDC".
func (*CDC) Type() string { return "CDC" }

// Type returns "TypeSelector".
func (*TypeSelector) Type() string { return "TypeSelector" }

// Type returns "IdSelector".
func (*IdSelector) Type() string { return "IdSelector" }

// Type returns "ClassSelector".
func (*ClassSelector) Type() string { return "ClassSelector" }

// Type returns "AttributeSelector".
func (*AttributeSelector) Type() string { return "AttributeSelector" }

// Type returns "PseudoClassSelector".
func (*PseudoClassSelector) Type() string { return "PseudoClassSelector" }

// Type returns "PseudoElementSelector".
func (*PseudoElementSelector) Type() string { return "PseudoElementSelector" }

// Type returns "Combinator".
func (*Combinator) Type() string { return "Combinator" }

// Type returns "Nth".
func (*Nth) Type() string { return "Nth" }

// Type returns "AnPlusB".
func (*AnPlusB) Type() string { return "AnPlusB" }

// Type returns "MediaQueryList".
func (*MediaQueryList) Type() string { return "MediaQueryList" }

// Type returns "MediaQuery".
func (*MediaQuery) Type() string { return "MediaQuery" }

// Type returns "MediaFeature".
func (*MediaFeature) Type() string { return "MediaFeature" }

// Type returns "Ratio".
func (*Ratio) Type() string { return "Ratio" }

// Type returns "UnicodeRange".
func (*UnicodeRange) Type() string { return "UnicodeRange" }

// Children returns the child list of nodes that have one (StyleSheet,
// AtrulePrelude, SelectorList, Selector, Block, DeclarationList, Value,
// Function, Parentheses, Brackets, the pseudo selectors, MediaQueryList and
// MediaQuery), and nil for every other node or a non-functional pseudo
// selector.
func Children(n Node) *List {
	switch n := n.(type) {
	case *StyleSheet:
		return n.Children
	case *AtrulePrelude:
		return n.Children
	case *SelectorList:
		return n.Children
	case *Selector:
		return n.Children
	case *Block:
		return n.Children
	case *DeclarationList:
		return n.Children
	case *Value:
		return n.Children
	case *Function:
		return n.Children
	case *Parentheses:
		return n.Children
	case *Brackets:
		return n.Children
	case *PseudoClassSelector:
		return n.Children
	case *PseudoElementSelector:
		return n.Children
	case *MediaQueryList:
		return n.Children
	case *MediaQuery:
		return n.Children
	}
	return nil
}
