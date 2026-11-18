package csso

import (
	"github.com/css/csso-go/internal/csstree"
)

// AST is a parsed CSS tree (csso's syntax AST), produced by Parse and
// Compress.
type AST struct {
	node csstree.Node
}

// Type returns the css-tree type of the root node, such as "StyleSheet" or
// "Value".
func (a *AST) Type() string {
	return a.node.Type()
}

// Generate serializes the tree back into CSS (csso's syntax.generate).
func (a *AST) Generate() string {
	return csstree.Generate(a.node)
}

// JSON returns the tree as JSON.stringify renders css-tree's node objects:
// the same keys in the same order, `loc` objects (or null) and lists as
// arrays. csso's compression annotations are not included.
func (a *AST) JSON() string {
	return csstree.JSON(a.node)
}

// ParseOptions configures Parse (css-tree's parse options). The zero value
// parses a stylesheet without positions.
type ParseOptions struct {
	// Context names the grammar production to parse: "stylesheet" (the
	// default), "atrule", "atrulePrelude", "mediaQueryList", "mediaQuery",
	// "rule", "selectorList", "selector", "block", "declarationList",
	// "declaration" or "value".
	Context string
	// Atrule is the at-rule name whose prelude grammar the
	// "atrulePrelude" context uses.
	Atrule string
	// Positions attaches source locations to the nodes.
	Positions bool
	// Filename is the source name stored in locations; "" means
	// "<unknown>".
	Filename string
	// Offset, Line and Column give the position of the first character of
	// source; zero Line and Column mean 1.
	Offset int
	Line   int
	Column int
	// RawAtrulePrelude, RawRulePrelude and RawValue keep at-rule preludes,
	// selectors and declaration values as unparsed text (css-tree's
	// parseAtrulePrelude, parseRulePrelude and parseValue set to false).
	RawAtrulePrelude bool
	RawRulePrelude   bool
	RawValue         bool
	// ParseCustomProperty parses the values of custom properties instead of
	// keeping them as text.
	ParseCustomProperty bool
	// OnParseError, when set, is called for every syntax error the parser
	// recovers from by keeping the broken part as text.
	OnParseError func(err *Error)
}

// parseContexts are the contexts css-tree's parser accepts.
var parseContexts = map[string]bool{
	csstree.ContextDefault: true, csstree.ContextStylesheet: true, csstree.ContextAtrule: true,
	csstree.ContextAtrulePrelude: true, csstree.ContextMediaQueryList: true, csstree.ContextMediaQuery: true,
	csstree.ContextRule: true, csstree.ContextSelectorList: true, csstree.ContextSelector: true,
	csstree.ContextBlock: true, csstree.ContextDeclarationList: true, csstree.ContextDeclaration: true,
	csstree.ContextValue: true,
}

// Parse parses CSS with csso's fork of css-tree (csso's syntax.parse),
// which generates strings and URLs in their shortest form. Recoverable
// syntax errors leave unparsed Raw nodes in the tree; other syntax errors
// are returned as an *Error named "SyntaxError". opts is optional; only its
// first element is used.
func Parse(source string, opts ...ParseOptions) (*AST, error) {
	var o ParseOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	context := o.Context
	if context == "" {
		context = csstree.ContextDefault
	}
	if !parseContexts[context] {
		return nil, &Error{Name: "Error", Message: "Unknown context `" + context + "`"}
	}

	parseOptions := &csstree.ParseOptions{
		Context:             context,
		Atrule:              o.Atrule,
		Positions:           o.Positions,
		Filename:            o.Filename,
		Offset:              o.Offset,
		Line:                o.Line,
		Column:              o.Column,
		RawAtrulePrelude:    o.RawAtrulePrelude,
		RawRulePrelude:      o.RawRulePrelude,
		RawValue:            o.RawValue,
		ParseCustomProperty: o.ParseCustomProperty,
	}
	if o.OnParseError != nil {
		parseOptions.OnParseError = func(err *csstree.SyntaxError, _ csstree.Node) {
			o.OnParseError(&Error{Name: "SyntaxError", Message: err.Message})
		}
	}

	node, err := csstree.Parse(source, parseOptions)
	if err != nil {
		return nil, convertError(err)
	}
	return &AST{node: node}, nil
}

// Compress minifies a parsed tree (csso's syntax.compress) and returns the
// compressed tree. The tree is changed in place unless Options.Clone is set;
// a nil ast stands for an empty stylesheet. Options.SourceMap and
// Options.Filename don't apply here. opts is optional; only its first
// element is used.
//
// A tree that is not a stylesheet is compressed as the block of a rule, as
// csso does; restructuring then fails, like in csso, for anything but a
// declaration list or a block.
func Compress(ast *AST, opts ...Options) (*AST, error) {
	var root csstree.Node
	if ast != nil {
		root = ast.node
	}
	node, err := compress(root, optionsOf(opts))
	if err != nil {
		return nil, err
	}
	return &AST{node: node}, nil
}
