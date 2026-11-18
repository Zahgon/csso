package restructure

import (
	"github.com/css/csso-go/internal/csstree"
)

// Sides of a box property, in shorthand value order.
const (
	top = iota
	right
	bottom
	left
)

// sideOf maps a longhand property to the side it sets.
var sideOf = map[string]int{
	"margin-top": top, "margin-right": right, "margin-bottom": bottom, "margin-left": left,
	"padding-top": top, "padding-right": right, "padding-bottom": bottom, "padding-left": left,
	"border-top-color": top, "border-right-color": right, "border-bottom-color": bottom, "border-left-color": left,
	"border-top-width": top, "border-right-width": right, "border-bottom-width": bottom, "border-left-width": left,
	"border-top-style": top, "border-right-style": right, "border-bottom-style": bottom, "border-left-style": left,
}

// mainProperty maps the box properties to their shorthand.
var mainProperty = map[string]string{
	"margin": "margin", "margin-top": "margin", "margin-right": "margin", "margin-bottom": "margin", "margin-left": "margin",
	"padding": "padding", "padding-top": "padding", "padding-right": "padding", "padding-bottom": "padding", "padding-left": "padding",
	"border-color": "border-color", "border-top-color": "border-color", "border-right-color": "border-color",
	"border-bottom-color": "border-color", "border-left-color": "border-color",
	"border-width": "border-width", "border-top-width": "border-width", "border-right-width": "border-width",
	"border-bottom-width": "border-width", "border-left-width": "border-width",
	"border-style": "border-style", "border-top-style": "border-style", "border-right-style": "border-style",
	"border-bottom-style": "border-style", "border-left-style": "border-style",
}

// sideValue is the value of one side of a TRBL shorthand. special names
// what prevents mixing it with other values (a CSS-wide keyword, a risky
// unit or a function name); "" when nothing does.
type sideValue struct {
	node      csstree.Node
	special   string
	important string
}

// trbl collects the four sides of a margin, padding or border-* shorthand
// from its declarations.
type trbl struct {
	name  string
	loc   *csstree.Location
	sides [4]*sideValue
	// iehack is the IE hack (\0 or \9) of the collected values; iehackSet
	// is false until a value sequence was read.
	iehack    string
	iehackSet bool
}

// getValueSequence returns the side values of a declaration, at most count,
// or nil when the value can't take part in a shorthand.
func (t *trbl) getValueSequence(declaration *csstree.Declaration, count int) []*sideValue {
	value, ok := declaration.Value.(*csstree.Value)
	if !ok {
		return nil
	}

	var values []*sideValue
	iehack := ""
	hasBadValues := value.Children.Some(func(child csstree.Node, _ *csstree.ListItem) bool {
		special := ""

		switch n := child.(type) {
		case *csstree.Identifier:
			switch n.Name {
			case `\0`, `\9`:
				iehack = n.Name
				return false
			case "inherit", "initial", "unset", "revert":
				special = n.Name
			}

		case *csstree.Dimension:
			switch n.Unit {
			// rem is not supported until IE11 and the viewport units are too
			// buggy across browsers ("vm" is IE9's "vmin")
			case "rem", "vw", "vh", "vmin", "vmax", "vm":
				special = n.Unit
			}

		case *csstree.Hash, *csstree.Number, *csstree.Percentage:

		case *csstree.Function:
			if n.Name == "var" {
				return true
			}
			special = n.Name

		default:
			return true // bad value
		}

		values = append(values, &sideValue{node: child, special: special, important: declaration.Important})
		return false
	})

	if hasBadValues || len(values) > count {
		return nil
	}

	if t.iehackSet && t.iehack != iehack {
		return nil
	}
	t.iehack, t.iehackSet = iehack, true

	return values
}

// canOverride reports whether value may replace the current value of side.
func (t *trbl) canOverride(side int, value *sideValue) bool {
	current := t.sides[side]
	return current == nil || (value.important != "" && current.important == "")
}

// mixable reports whether value can be combined with the sides collected
// so far: their special markers must be equal.
func (t *trbl) mixable(value *sideValue) bool {
	for _, side := range t.sides {
		if side != nil && side.special != value.special {
			return false
		}
	}
	return true
}

// add merges the declaration of property name into the shorthand and
// reports whether it could.
func (t *trbl) add(name string, declaration *csstree.Declaration) bool {
	if !t.attemptToAdd(name, declaration) {
		return false
	}
	if t.loc == nil {
		t.loc = declaration.Loc()
	}
	return true
}

func (t *trbl) attemptToAdd(name string, declaration *csstree.Declaration) bool {
	if side, ok := sideOf[name]; ok {
		values := t.getValueSequence(declaration, 1)
		if len(values) == 0 || !t.mixable(values[0]) {
			return false
		}
		if t.canOverride(side, values[0]) {
			t.sides[side] = values[0]
		}
		return true
	}

	if name != t.name {
		return false
	}

	values := t.getValueSequence(declaration, 4)
	if len(values) == 0 {
		return false
	}

	// expand to four sides, as the shorthand syntax does
	switch len(values) {
	case 1:
		values = append(values, values[top], values[top], values[top])
	case 2:
		values = append(values, values[top], values[right])
	case 3:
		values = append(values, values[right])
	}

	for _, value := range values {
		if !t.mixable(value) {
			return false
		}
	}

	for side, value := range values {
		if t.canOverride(side, value) {
			t.sides[side] = value
		}
	}
	return true
}

// isOkToMinimize reports whether all four sides are known and are all
// !important or all not.
func (t *trbl) isOkToMinimize() bool {
	for _, side := range t.sides {
		if side == nil {
			return false
		}
	}
	// csso sums the important flags and wants 0 or 4: a hack such as !ie
	// never qualifies
	important := t.sides[top].important
	for _, side := range t.sides {
		if side.important != important || (important != "" && important != "important") {
			return false
		}
	}
	return true
}

// getValue returns the shortest shorthand value for the four sides.
func (t *trbl) getValue() *csstree.Value {
	values := t.sides[:]
	var stringValues [4]string
	for i, side := range t.sides {
		stringValues[i] = csstree.Generate(side.node)
	}

	if stringValues[left] == stringValues[right] {
		values = values[:3]
		if stringValues[bottom] == stringValues[top] {
			values = values[:2]
			if stringValues[right] == stringValues[top] {
				values = values[:1]
			}
		}
	}

	result := csstree.NewList()
	for _, value := range values {
		result.AppendData(value.node)
	}
	if t.iehack != "" {
		result.AppendData(&csstree.Identifier{Name: t.iehack})
	}

	return &csstree.Value{Children: result}
}

// getDeclaration returns the shorthand declaration.
func (t *trbl) getDeclaration() *csstree.Declaration {
	return &csstree.Declaration{
		Base:      csstree.Base{Location: t.loc},
		Important: t.sides[top].important,
		Property:  t.name,
		Value:     t.getValue(),
	}
}

// shorthandOperation tells what to do with a declaration once its shorthand
// is complete: replace it by the shorthand, or remove it.
type shorthandOperation int

const (
	replaceDeclaration shorthandOperation = iota + 1
	removeDeclaration
)

type shortDeclaration struct {
	operation shorthandOperation
	block     *csstree.List
	item      *csstree.ListItem
	shorthand *trbl
}

// collectShorthands feeds the box declarations of a rule, last first, into
// the shorthands of shorts; consecutive rules with the same selector share
// shorthands. It returns the selector shorthands were last collected for.
func collectShorthands(rule *csstree.Rule, shorts map[string]*trbl, shortDeclarations *[]shortDeclaration, lastShortSelector string) string {
	declarations := rule.Block.Children
	selectorID := selector(selectorList(rule).First()).ID

	declarations.ForEachRight(func(node csstree.Node, item *csstree.ListItem) {
		declaration, ok := node.(*csstree.Declaration)
		if !ok {
			return
		}
		property := declaration.Property
		key, ok := mainProperty[property]
		if !ok {
			return
		}

		var shorthand *trbl
		var operation shorthandOperation

		if lastShortSelector == "" || selectorID == lastShortSelector {
			if existing, ok := shorts[key]; ok {
				operation = removeDeclaration
				shorthand = existing
			}
		}

		if shorthand == nil || !shorthand.add(property, declaration) {
			operation = replaceDeclaration
			shorthand = &trbl{name: key}

			// a value that can't be parsed breaks the shorthand
			if !shorthand.add(property, declaration) {
				lastShortSelector = ""
				return
			}
		}

		shorts[key] = shorthand
		*shortDeclarations = append(*shortDeclarations, shortDeclaration{
			operation: operation,
			block:     declarations,
			item:      item,
			shorthand: shorthand,
		})

		lastShortSelector = selectorID
	})

	return lastShortSelector
}

// restructShorthand replaces complete sets of margin, padding and
// border-color/-width/-style declarations by their shorthand (csso
// restructure/4-restructShorthand.js).
func restructShorthand(ast *csstree.StyleSheet, indexer *declarationIndexer) {
	type ruleGroup struct {
		lastShortSelector string
		shorts            map[string]map[string]*trbl
	}
	stylesheetMap := make(map[int]*ruleGroup)
	var shortDeclarations []shortDeclaration

	csstree.Walk(ast, csstree.WalkOptions{
		Visit:   "Rule",
		Reverse: true,
		Enter: func(ctx *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			ruleID := rule.PseudoSignature + "|" + selector(selectorList(rule).First()).ID

			group, ok := stylesheetMap[containerID(ctx)]
			if !ok {
				group = &ruleGroup{shorts: make(map[string]map[string]*trbl)}
				stylesheetMap[containerID(ctx)] = group
			}

			shorts, ok := group.shorts[ruleID]
			if !ok {
				shorts = make(map[string]*trbl)
				group.shorts[ruleID] = shorts
			}

			group.lastShortSelector = collectShorthands(rule, shorts, &shortDeclarations, group.lastShortSelector)
			return csstree.WalkContinue
		},
	})

	for _, short := range shortDeclarations {
		if !short.shorthand.isOkToMinimize() {
			continue
		}
		if short.operation == replaceDeclaration {
			short.item.Data = indexer.mark(short.shorthand.getDeclaration())
		} else {
			short.block.Remove(short.item)
		}
	}
}

// containerID identifies the block or stylesheet that holds the rule being
// visited (csso's `(this.block || this.stylesheet).id`).
func containerID(ctx *csstree.WalkContext) int {
	if ctx.Block != nil {
		return ctx.Block.ID
	}
	return ctx.Stylesheet.ID
}
