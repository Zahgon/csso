// Package clean removes redundant nodes from a stylesheet: comments,
// whitespace, empty rules and at-rules, misplaced @charset/@import, useless
// universal selectors and, with usage data, unused selectors (csso
// lib/clean).
package clean

import (
	"strings"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csso/nodes"
	"github.com/css/csso-go/internal/csso/usage"
	"github.com/css/csso-go/internal/csstree"
)

// Options configures a clean pass.
type Options struct {
	// Usage filters selectors by the names in use; nil disables filtering.
	Usage *usage.Index
	// Detached is a block that stands in for a wrapped root node which is
	// not a Block (csso compresses a DeclarationList or a Value by making it
	// the block of a dummy rule). css-tree's walker does not record such a
	// node as the enclosing block, so Raw nodes in it are kept.
	Detached *csstree.Block
}

type cleaner struct {
	opts       Options
	stylesheet *csstree.StyleSheet
	err        error
}

// Run cleans the stylesheet ast in place.
func Run(ast *csstree.StyleSheet, opts Options) error {
	return RunNode(ast, ast, opts)
}

// RunNode cleans the tree rooted at root, which is part of the stylesheet
// being compressed. Handlers that would remove a root node (one that is not
// in a list) fail with the TypeError csso throws in that case.
func RunNode(root csstree.Node, stylesheet *csstree.StyleSheet, opts Options) error {
	c := &cleaner{opts: opts, stylesheet: stylesheet}
	csstree.Walk(root, csstree.WalkOptions{
		Leave: func(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			if err := c.leave(ctx, node, item, list); err != nil {
				c.err = err
				return csstree.WalkBreak
			}
			return csstree.WalkContinue
		},
	})
	return c.err
}

func (c *cleaner) leave(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) error {
	switch n := node.(type) {
	case *csstree.Atrule:
		return c.atrule(ctx, n, item, list)
	case *csstree.Comment, *csstree.WhiteSpace:
		return remove(list, item)
	case *csstree.Declaration:
		return declaration(n, item, list)
	case *csstree.Raw:
		c.raw(ctx, item, list)
	case *csstree.Rule:
		return c.rule(ctx, n, item, list)
	case *csstree.TypeSelector:
		typeSelector(n, item, list)
	}
	return nil
}

// remove drops item from list. A node walked as a field rather than a list
// item has no list, and csso's `list.remove(item)` then throws.
func remove(list *csstree.List, item *csstree.ListItem) error {
	if list == nil {
		return jscompat.ReadPropertyError("undefined", "remove")
	}
	list.Remove(item)
	return nil
}

// atrule drops at-rules with empty blocks, a @charset that is empty or not
// first, an @import after other rules, and @keyframes, @media or @supports
// without prelude or rules.
func (c *cleaner) atrule(ctx *csstree.WalkContext, node *csstree.Atrule, item *csstree.ListItem, list *csstree.List) error {
	if node.Block != nil {
		// a removed at-rule with a block still prevents a later @import
		c.stylesheet.FirstAtrulesAllowed = false

		if nodes.HasNoChildren(node.Block) {
			return remove(list, item)
		}
	}

	switch node.Name {
	case "charset":
		if nodes.HasNoChildren(node.Prelude) {
			return remove(list, item)
		}
		if item == nil {
			return jscompat.ReadPropertyError("undefined", "prev")
		}
		// any rule before @charset makes it invalid
		if item.Prev != nil {
			list.Remove(item)
		}

	case "import":
		if !c.stylesheet.FirstAtrulesAllowed {
			return remove(list, item)
		}
		if item == nil {
			return jscompat.ReadPropertyError("undefined", "prev")
		}
		// an @import preceded by anything but @import or @charset is ignored
		// by browsers
		list.PrevUntil(item.Prev, func(prev csstree.Node, _ *csstree.ListItem) bool {
			if rule, ok := prev.(*csstree.Atrule); ok && (rule.Name == "import" || rule.Name == "charset") {
				return false
			}
			if root, ok := ctx.Root.(*csstree.StyleSheet); ok {
				root.FirstAtrulesAllowed = false
			}
			list.Remove(item)
			return true
		})

	default:
		switch csstree.Keyword(node.Name).Basename {
		case "keyframes", "media", "supports":
			if nodes.HasNoChildren(node.Prelude) || nodes.HasNoChildren(nodes.Block(node.Block)) {
				return remove(list, item)
			}
		}
	}
	return nil
}

// declaration drops declarations with an empty value and trims the raw
// value of custom properties.
func declaration(node *csstree.Declaration, item *csstree.ListItem, list *csstree.List) error {
	if value, ok := node.Value.(*csstree.Value); ok && value.Children.IsEmpty() {
		return remove(list, item)
	}

	if csstree.Property(node.Property).Custom {
		raw, ok := node.Value.(*csstree.Raw)
		if !ok {
			// csso trims `value.value`, which only a Raw value has
			return jscompat.ReadPropertyError("undefined", "trim")
		}
		if jscompat.HasNonSpace(raw.Value) {
			raw.Value = jscompat.TrimSpace(raw.Value)
		}
	}
	return nil
}

// raw drops unparsed text standing directly in the stylesheet or a block.
func (c *cleaner) raw(ctx *csstree.WalkContext, item *csstree.ListItem, list *csstree.List) {
	if list == nil {
		return
	}
	if list == c.stylesheet.Children ||
		(ctx.Block != nil && ctx.Block != c.opts.Detached && list == ctx.Block.Children) {
		list.Remove(item)
	}
}

// rule drops rules without selectors or declarations and, with usage data,
// the selectors that use unknown names.
func (c *cleaner) rule(ctx *csstree.WalkContext, node *csstree.Rule, item *csstree.ListItem, list *csstree.List) error {
	if nodes.HasNoChildren(node.Prelude) || nodes.HasNoChildren(nodes.Block(node.Block)) {
		return remove(list, item)
	}

	// keyframe selectors are not filtered
	if ctx.Atrule != nil && csstree.Keyword(ctx.Atrule.Name).Basename == "keyframes" {
		return nil
	}

	u := c.opts.Usage
	if u == nil || (u.Whitelist == nil && u.Blacklist == nil) {
		return nil
	}
	if selectors, ok := node.Prelude.(*csstree.SelectorList); ok {
		cleanUnused(selectors, u)
	}
	if nodes.HasNoChildren(node.Prelude) {
		return remove(list, item)
	}
	return nil
}

// cleanUnused removes the selectors of selectorList that use a tag, id or
// class missing from the whitelist or present in the blacklist, and reports
// whether the list became empty. Nested selector lists are filtered too,
// except inside :not().
func cleanUnused(selectorList *csstree.SelectorList, u *usage.Index) bool {
	selectorList.Children.ForEach(func(selector csstree.Node, item *csstree.ListItem) {
		shouldRemove := false

		csstree.Walk(selector, csstree.WalkOptions{
			Enter: func(ctx *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
				// ignore nodes in nested selectors
				if ctx.Selector != nil && ctx.Selector != selectorList {
					return csstree.WalkContinue
				}

				switch n := node.(type) {
				case *csstree.SelectorList:
					if ctx.Function == nil || csstree.LowerCase(nodes.FunctionName(ctx.Function)) != "not" {
						if cleanUnused(n, u) {
							shouldRemove = true
						}
					}
				case *csstree.ClassSelector:
					if unused(u, n.Name, func(l *usage.List) usage.Names { return l.Classes }) {
						shouldRemove = true
					}
				case *csstree.IdSelector:
					if unused(u, n.Name, func(l *usage.List) usage.Names { return l.IDs }) {
						shouldRemove = true
					}
				case *csstree.TypeSelector:
					// universal selectors are always in use
					if !strings.HasSuffix(n.Name, "*") &&
						unused(u, csstree.LowerCase(n.Name), func(l *usage.List) usage.Names { return l.Tags }) {
						shouldRemove = true
					}
				}
				return csstree.WalkContinue
			},
		})

		if shouldRemove {
			selectorList.Children.Remove(item)
		}
	})

	return selectorList.Children.IsEmpty()
}

// unused reports whether name is missing from the whitelist's names or
// present in the blacklist's names of the kind names selects.
func unused(u *usage.Index, name string, names func(*usage.List) usage.Names) bool {
	if u.Whitelist != nil {
		if set := names(u.Whitelist); set != nil && !set[name] {
			return true
		}
	}
	if u.Blacklist != nil {
		if set := names(u.Blacklist); set != nil && set[name] {
			return true
		}
	}
	return false
}

// typeSelector drops a non-namespaced universal selector followed by an id,
// class, attribute or pseudo selector.
func typeSelector(node *csstree.TypeSelector, item *csstree.ListItem, list *csstree.List) {
	if node.Name != "*" || item == nil || item.Next == nil {
		return
	}
	switch item.Next.Data.(type) {
	case *csstree.IdSelector, *csstree.ClassSelector, *csstree.AttributeSelector,
		*csstree.PseudoClassSelector, *csstree.PseudoElementSelector:
		list.Remove(item)
	}
}
