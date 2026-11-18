package csstree

import (
	"fmt"
	"slices"
	"strings"
)

// WalkAction tells Walk how to continue after a callback.
type WalkAction int

const (
	// WalkContinue proceeds normally.
	WalkContinue WalkAction = iota
	// WalkSkip, returned from Enter, skips the node's children and its
	// Leave callback (css-tree's walk.skip).
	WalkSkip
	// WalkBreak stops the whole walk (css-tree's walk.break).
	WalkBreak
)

// WalkContext exposes the enclosing nodes of the node being visited, like
// the `this` object of css-tree walker callbacks. Each field holds the
// nearest enclosing node of that kind (including the visited node itself
// once its children are being walked), or nil.
type WalkContext struct {
	Root          Node
	Stylesheet    *StyleSheet
	Atrule        *Atrule
	AtrulePrelude *AtrulePrelude
	Rule          *Rule
	Selector      *SelectorList
	Block         *Block
	Declaration   *Declaration
	// Function is the enclosing *Function, *PseudoClassSelector or
	// *PseudoElementSelector.
	Function Node
}

// WalkFunc is a walker callback. item and list are the list item holding
// node and its list, or nil when node is not a list element.
type WalkFunc func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction

// WalkOptions configures Walk.
type WalkOptions struct {
	// Enter is called before a node's children are walked.
	Enter WalkFunc
	// Leave is called after a node's children were walked.
	Leave WalkFunc
	// Visit restricts the callbacks to nodes of this type. For "Atrule",
	// "Rule" and "Declaration" the walk only descends into the nodes that
	// can contain them, as in css-tree.
	Visit string
	// Reverse walks children (and node fields) in reverse order.
	Reverse bool
}

// nodeTypes lists every node type name, for validating WalkOptions.Visit.
var nodeTypes = []string{
	"AnPlusB", "Atrule", "AtrulePrelude", "AttributeSelector", "Block", "Brackets",
	"CDC", "CDO", "ClassSelector", "Combinator", "Comment", "Declaration",
	"DeclarationList", "Dimension", "Function", "Hash", "IdSelector", "Identifier",
	"MediaFeature", "MediaQuery", "MediaQueryList", "Nth", "Number", "Operator",
	"Parentheses", "Percentage", "PseudoClassSelector", "PseudoElementSelector",
	"Ratio", "Raw", "Rule", "Selector", "SelectorList", "String", "StyleSheet",
	"TypeSelector", "UnicodeRange", "Url", "Value", "WhiteSpace",
}

type walker struct {
	ctx     WalkContext
	enter   WalkFunc
	leave   WalkFunc
	reverse bool
	// descend reports whether the children of a node type are walked.
	descend func(nodeType string) bool
}

// Walk traverses the tree rooted at root depth-first, calling opts.Enter
// and opts.Leave for every node. Lists may be mutated from the callbacks
// (List iteration is cursor-safe). Walk panics when neither callback is set
// or Visit names an unknown node type, which are programming errors.
func Walk(root Node, opts WalkOptions) {
	w := &walker{
		enter:   opts.Enter,
		leave:   opts.Leave,
		reverse: opts.Reverse,
		descend: func(string) bool { return true },
	}
	w.ctx.Root = root

	if w.enter == nil && w.leave == nil {
		panic("csstree: neither Enter nor Leave walker handler is set")
	}

	if opts.Visit != "" {
		switch opts.Visit {
		case "Atrule", "Rule":
			w.descend = func(t string) bool {
				return t == "StyleSheet" || t == "Atrule" || t == "Rule" || t == "Block"
			}
		case "Declaration":
			w.descend = func(t string) bool {
				return t == "StyleSheet" || t == "Atrule" || t == "Rule" || t == "Block" || t == "DeclarationList"
			}
		default:
			if !slices.Contains(nodeTypes, opts.Visit) {
				panic(fmt.Sprintf("csstree: bad value `%s` for `visit` option (should be: %s)",
					opts.Visit, strings.Join(nodeTypes, ", ")))
			}
		}
		w.enter = filterByType(w.enter, opts.Visit)
		w.leave = filterByType(w.leave, opts.Visit)
	}

	w.walkNode(root, nil, nil)
}

func filterByType(fn WalkFunc, nodeType string) WalkFunc {
	if fn == nil {
		return nil
	}
	return func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
		if node.Type() != nodeType {
			return WalkContinue
		}
		return fn(ctx, node, item, list)
	}
}

// walkNode visits node and reports whether the walk was broken.
func (w *walker) walkNode(node Node, item *ListItem, list *List) bool {
	if w.enter != nil {
		switch w.enter(&w.ctx, node, item, list) {
		case WalkBreak:
			return true
		case WalkSkip:
			return false
		}
	}

	if w.descend(node.Type()) && w.walkChildren(node) {
		return true
	}

	return w.leave != nil && w.leave(&w.ctx, node, item, list) == WalkBreak
}

// walkList walks the items of list with a cursor, so the callbacks may
// mutate it.
func (w *walker) walkList(list *List) bool {
	broken := false
	if w.reverse {
		c := list.allocateCursor(list.Tail, nil)
		for c.prev != nil && !broken {
			item := c.prev
			c.prev = item.Prev
			broken = w.walkNode(item.Data, item, list)
		}
	} else {
		c := list.allocateCursor(nil, list.Head)
		for c.next != nil && !broken {
			item := c.next
			c.next = item.Next
			broken = w.walkNode(item.Data, item, list)
		}
	}
	list.releaseCursor()
	return broken
}

// walkFields walks the node-valued fields of a node (nil entries are
// skipped) in order, or in reverse order for a reverse walk.
func (w *walker) walkFields(fields ...Node) bool {
	if w.reverse {
		slices.Reverse(fields)
	}
	for _, field := range fields {
		if field != nil && w.walkNode(field, nil, nil) {
			return true
		}
	}
	return false
}

// walkChildren walks the child fields of node, setting the walk context
// for its duration (css-tree's walkContext). On break the context is left
// as is, like css-tree does.
func (w *walker) walkChildren(node Node) bool {
	ctx := &w.ctx
	switch n := node.(type) {
	case *StyleSheet:
		prev := ctx.Stylesheet
		ctx.Stylesheet = n
		if w.walkList(n.Children) {
			return true
		}
		ctx.Stylesheet = prev
	case *Atrule:
		prev := ctx.Atrule
		ctx.Atrule = n
		if w.walkFields(n.Prelude, blockField(n.Block)) {
			return true
		}
		ctx.Atrule = prev
	case *AtrulePrelude:
		prev := ctx.AtrulePrelude
		ctx.AtrulePrelude = n
		if w.walkList(n.Children) {
			return true
		}
		ctx.AtrulePrelude = prev
	case *Rule:
		prev := ctx.Rule
		ctx.Rule = n
		if w.walkFields(n.Prelude, blockField(n.Block)) {
			return true
		}
		ctx.Rule = prev
	case *SelectorList:
		prev := ctx.Selector
		ctx.Selector = n
		if w.walkList(n.Children) {
			return true
		}
		ctx.Selector = prev
	case *Block:
		prev := ctx.Block
		ctx.Block = n
		if w.walkList(n.Children) {
			return true
		}
		ctx.Block = prev
	case *Declaration:
		prev := ctx.Declaration
		ctx.Declaration = n
		if w.walkFields(n.Value) {
			return true
		}
		ctx.Declaration = prev
	case *Function:
		return w.walkFunctionLike(n, n.Children)
	case *PseudoClassSelector:
		return w.walkFunctionLike(n, n.Children)
	case *PseudoElementSelector:
		return w.walkFunctionLike(n, n.Children)
	case *AttributeSelector:
		var name Node
		if n.Name != nil {
			name = n.Name
		}
		return w.walkFields(name, n.Value)
	case *MediaFeature:
		return w.walkFields(n.Value)
	case *Nth:
		var selector Node
		if n.Selector != nil {
			selector = n.Selector
		}
		return w.walkFields(n.Nth, selector)
	default:
		if children := Children(node); children != nil {
			return w.walkList(children)
		}
	}
	return false
}

func (w *walker) walkFunctionLike(node Node, children *List) bool {
	prev := w.ctx.Function
	w.ctx.Function = node
	if children != nil && w.walkList(children) {
		return true
	}
	w.ctx.Function = prev
	return false
}

// blockField converts a possibly nil *Block into a Node without creating a
// non-nil interface around a nil pointer.
func blockField(b *Block) Node {
	if b == nil {
		return nil
	}
	return b
}

// Find returns the first node (in walk order) for which fn returns true, or
// nil.
func Find(root Node, fn func(ctx *WalkContext, node Node, item *ListItem, list *List) bool) Node {
	var found Node
	Walk(root, WalkOptions{Enter: func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
		if fn(ctx, node, item, list) {
			found = node
			return WalkBreak
		}
		return WalkContinue
	}})
	return found
}

// FindLast returns the first node of a reverse walk for which fn returns
// true, or nil.
func FindLast(root Node, fn func(ctx *WalkContext, node Node, item *ListItem, list *List) bool) Node {
	var found Node
	Walk(root, WalkOptions{Reverse: true, Enter: func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
		if fn(ctx, node, item, list) {
			found = node
			return WalkBreak
		}
		return WalkContinue
	}})
	return found
}

// FindAll returns every node for which fn returns true, in walk order.
func FindAll(root Node, fn func(ctx *WalkContext, node Node, item *ListItem, list *List) bool) []Node {
	var found []Node
	Walk(root, WalkOptions{Enter: func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
		if fn(ctx, node, item, list) {
			found = append(found, node)
		}
		return WalkContinue
	}})
	return found
}
