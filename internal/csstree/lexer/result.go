package lexer

import (
	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// MatchNode is a node of the match tree css-tree's matchAsTree builds: either
// a syntax (the root, or a Type/Property reference that was entered) with the
// matches inside it, or a matched token.
type MatchNode struct {
	// Syntax is the definition the match belongs to: for a syntax node the
	// opened *defsyntax.Type or *defsyntax.Property (the root holds the
	// matched descriptor's reference, nil for an anonymous one); for a token
	// the definition term that consumed it (nil for generic types).
	Syntax defsyntax.Node
	// Children are the matches within a syntax node.
	Children []*MatchNode
	// Token is the value of a matched token.
	Token string
	// Node is the AST node whose generation emitted a matched token, nil for
	// tokens not attributed to a node.
	Node csstree.Node

	isToken bool
}

// IsToken reports whether m is a matched token rather than a syntax node.
func (m *MatchNode) IsToken() bool { return m.isToken }

// MatchResult is the outcome of matching a value against a syntax, with
// css-tree's trace helpers.
type MatchResult struct {
	// Matched is the match tree, nil when matching failed.
	Matched *MatchNode
	// Iterations is the number of matcher steps taken.
	Iterations int
	// Error explains why matching failed or was not attempted.
	Error error
}

func failedMatch(err error) *MatchResult {
	return &MatchResult{Error: err}
}

// matchAsTree runs the matcher and turns the resulting match stack into a
// tree rooted at the graph's syntax reference.
func matchAsTree(tokens []token, graph *state, l *Lexer) (*matchOutcome, *MatchNode, error) {
	outcome, err := internalMatch(tokens, graph, l)
	if err != nil || outcome.match == nil {
		return outcome, nil, err
	}

	// the stack is linked from the last item; its first item is a stub
	var items []*matchItem
	for item := outcome.match; item.prev != nil; item = item.prev {
		items = append(items, item)
	}

	root := &MatchNode{Syntax: graph.syntax, Children: []*MatchNode{}}
	host := root
	hostStack := []*MatchNode{root}

	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		switch item.kind {
		case itemOpenSyntax:
			child := &MatchNode{Syntax: item.syntax, Children: []*MatchNode{}}
			host.Children = append(host.Children, child)
			host = child
			hostStack = append(hostStack, host)
		case itemCloseSyntax:
			hostStack = hostStack[:len(hostStack)-1]
			host = hostStack[len(hostStack)-1]
		default:
			host.Children = append(host.Children, &MatchNode{
				Syntax:  item.syntax,
				Token:   item.token.value,
				Node:    item.token.node,
				isToken: true,
			})
		}
	}

	return outcome, root, nil
}

// shouldPutToTrace reports whether a syntax appears in traces: Type, Property
// and Keyword definitions do.
func shouldPutToTrace(syntax defsyntax.Node) bool {
	switch syntax.(type) {
	case *defsyntax.Type, *defsyntax.Property, *defsyntax.Keyword:
		return true
	}
	return false
}

// GetTrace returns the Type, Property and Keyword syntaxes, outermost first,
// that the first token emitted by node was matched through. It returns nil
// when matching failed or node emitted no matched token, and an empty slice
// when the token was matched outside of any traced syntax.
func (r *MatchResult) GetTrace(node csstree.Node) []defsyntax.Node {
	if r.Matched == nil {
		return nil
	}

	var trace []defsyntax.Node
	var hasMatch func(m *MatchNode) bool
	hasMatch = func(m *MatchNode) bool {
		if !m.isToken {
			for _, child := range m.Children {
				if hasMatch(child) {
					if shouldPutToTrace(m.Syntax) {
						trace = append(trace, m.Syntax)
					}
					return true
				}
			}
			return false
		}
		if m.Node == node {
			trace = []defsyntax.Node{}
			if shouldPutToTrace(m.Syntax) {
				trace = append(trace, m.Syntax)
			}
			return true
		}
		return false
	}

	if !hasMatch(r.Matched) {
		return nil
	}
	// the trace was collected innermost first
	for i, j := 0, len(trace)-1; i < j; i, j = i+1, j-1 {
		trace[i], trace[j] = trace[j], trace[i]
	}
	return trace
}

func (r *MatchResult) testNode(node csstree.Node, fn func(defsyntax.Node) bool) bool {
	for _, syntax := range r.GetTrace(node) {
		if fn(syntax) {
			return true
		}
	}
	return false
}

// IsType reports whether node was matched as (part of) the named type, e.g.
// IsType(ident, "color") for a color keyword.
func (r *MatchResult) IsType(node csstree.Node, typeName string) bool {
	return r.testNode(node, func(syntax defsyntax.Node) bool {
		t, ok := syntax.(*defsyntax.Type)
		return ok && t.Name == typeName
	})
}

// IsProperty reports whether node was matched as (part of) the value of the
// named property.
func (r *MatchResult) IsProperty(node csstree.Node, property string) bool {
	return r.testNode(node, func(syntax defsyntax.Node) bool {
		p, ok := syntax.(*defsyntax.Property)
		return ok && p.Name == property
	})
}

// IsKeyword reports whether node was matched as a keyword of a syntax.
func (r *MatchResult) IsKeyword(node csstree.Node) bool {
	return r.testNode(node, func(syntax defsyntax.Node) bool {
		_, ok := syntax.(*defsyntax.Keyword)
		return ok
	})
}
