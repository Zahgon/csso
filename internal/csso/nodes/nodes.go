// Package nodes holds small css-tree node helpers shared by csso's passes.
package nodes

import (
	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// HasNoChildren reports whether node is absent, has no child list or an
// empty one (csso's clean/utils hasNoChildren).
func HasNoChildren(node csstree.Node) bool {
	if node == nil {
		return true
	}
	children := csstree.Children(node)
	return children == nil || children.IsEmpty()
}

// Block converts a possibly nil *Block into a Node without wrapping a nil
// pointer in a non-nil interface.
func Block(b *csstree.Block) csstree.Node {
	if b == nil {
		return nil
	}
	return b
}

// FunctionName returns the name of a function-like node, the kind css-tree's
// walker records as the enclosing function: a Function or a pseudo-class or
// pseudo-element selector. It returns "" for any other node.
func FunctionName(node csstree.Node) string {
	switch n := node.(type) {
	case *csstree.Function:
		return n.Name
	case *csstree.PseudoClassSelector:
		return n.Name
	case *csstree.PseudoElementSelector:
		return n.Name
	}
	return ""
}

// ChildList returns the children of node the way csso reads `node.children`
// before iterating it: a node type without a child list fails with the
// TypeError for undefined, a pseudo selector without arguments with the one
// for null.
func ChildList(node csstree.Node, method string) (*csstree.List, error) {
	if children := csstree.Children(node); children != nil {
		return children, nil
	}
	switch node.(type) {
	case *csstree.PseudoClassSelector, *csstree.PseudoElementSelector:
		return nil, jscompat.ReadPropertyError("null", method)
	}
	return nil, jscompat.ReadPropertyError("undefined", method)
}
