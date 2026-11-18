// Package replace rewrites nodes into shorter equivalents: packed numbers,
// unitless zeros, short colors, unquoted attribute values, normalized
// keyframe selectors and shorter font, background and border values (csso
// lib/replace).
package replace

import (
	"regexp"
	"strings"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csso/nodes"
	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer"
)

// Run applies the replacements to the tree rooted at root, visiting nodes
// after their children.
func Run(root csstree.Node) error {
	var err error
	csstree.Walk(root, csstree.WalkOptions{
		Leave: func(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			if err = leave(ctx, node, item); err != nil {
				return csstree.WalkBreak
			}
			return csstree.WalkContinue
		},
	})
	return err
}

func leave(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem) error {
	switch n := node.(type) {
	case *csstree.Atrule:
		if csstree.Keyword(n.Name).Basename == "keyframes" {
			return keyframes(n)
		}
	case *csstree.AttributeSelector:
		attributeSelector(n)
	case *csstree.Value:
		value(ctx, n)
	case *csstree.Dimension:
		return dimension(ctx, n, item)
	case *csstree.Percentage:
		return percentage(ctx, n, item)
	case *csstree.Number:
		number(n)
	case *csstree.Url:
		// convert `\` to `/`
		n.Value = strings.ReplaceAll(n.Value, `\`, "/")
	case *csstree.Hash:
		return hexColor(n, item)
	case *csstree.Identifier:
		return identColor(ctx, n, item)
	case *csstree.Function:
		return colorFunction(ctx, n, item)
	}
	return nil
}

// setData replaces the node held by item. A node that is not a list item
// has no item, and csso's `item.data = ...` then throws.
func setData(item *csstree.ListItem, node csstree.Node) error {
	if item == nil {
		return jscompat.NewTypeError("Cannot set properties of undefined (setting 'data')")
	}
	item.Data = node
	return nil
}

// matchesType reports whether the value of decl matches its property syntax
// with node matched as (part of) typeName.
func matchesType(decl *csstree.Declaration, node csstree.Node, typeName string) bool {
	return lexer.Default().MatchDeclaration(decl).IsType(node, typeName)
}

// blockUnquote matches the attribute values that are not valid identifiers
// and must stay quoted (Mathias Bynens' unquoted-attributes check, as csso
// adopts it).
var blockUnquote = regexp.MustCompile(`^(-?[0-9]|--)|[\x{0000}-\x{002c}\x{002e}\x{002f}\x{003A}-\x{0040}\x{005B}-\x{005E}\x{0060}\x{007B}-\x{009f}]`)

// attributeSelector unquotes a string attribute value that is a valid
// identifier.
func attributeSelector(node *csstree.AttributeSelector) {
	attrValue, ok := node.Value.(*csstree.String)
	if !ok {
		return
	}
	if attrValue.Value == "" || attrValue.Value == "-" || blockUnquote.MatchString(attrValue.Value) {
		return
	}
	node.Value = &csstree.Identifier{Base: attrValue.Base, Name: attrValue.Value}
}

// keyframes writes the keyframe selector `100%` as `to` and `from` as `0%`.
// Like csso it expects every child of the block to have a prelude of
// selectors and fails with csso's TypeError otherwise.
func keyframes(node *csstree.Atrule) error {
	if node.Block == nil {
		return jscompat.ReadPropertyError("null", "children")
	}
	for rule := range node.Block.Children.All() {
		var prelude csstree.Node
		switch r := rule.(type) {
		case *csstree.Rule:
			prelude = r.Prelude
		case *csstree.Atrule:
			if r.Prelude == nil {
				return jscompat.ReadPropertyError("null", "children")
			}
			prelude = r.Prelude
		default:
			return jscompat.ReadPropertyError("undefined", "children")
		}

		selectors, err := nodes.ChildList(prelude, "forEach")
		if err != nil {
			return err
		}
		for selector := range selectors.All() {
			children, err := nodes.ChildList(selector, "forEach")
			if err != nil {
				return err
			}
			for item := children.Head; item != nil; item = item.Next {
				switch data := item.Data.(type) {
				case *csstree.Percentage:
					if data.Value == "100" {
						item.Data = &csstree.TypeSelector{Base: data.Base, Name: "to"}
					}
				case *csstree.TypeSelector:
					if data.Name == "from" {
						item.Data = &csstree.Percentage{Base: data.Base, Value: "0"}
					}
				}
			}
		}
	}
	return nil
}
