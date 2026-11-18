package restructure

import (
	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// selector returns the annotated selector held by a SelectorList item.
// Selector lists only hold Selector nodes once a rule survived cleaning;
// anything else reads as a selector without annotations.
func selector(node csstree.Node) *csstree.Selector {
	if s, ok := node.(*csstree.Selector); ok {
		return s
	}
	return &csstree.Selector{}
}

// declarationID returns csso's declaration identity of a block item: equal
// generated declarations share an id. Other nodes have id 0.
func declarationID(node csstree.Node) int {
	if d, ok := node.(*csstree.Declaration); ok {
		return d.ID
	}
	return 0
}

// selectorList returns the selectors of a rule.
func selectorList(rule *csstree.Rule) *csstree.List {
	return csstree.Children(rule.Prelude)
}

// isEqualSelectors reports whether two selector lists hold the same
// selectors in the same order.
func isEqualSelectors(a, b *csstree.List) bool {
	cursor1, cursor2 := a.Head, b.Head
	for cursor1 != nil && cursor2 != nil && selector(cursor1.Data).ID == selector(cursor2.Data).ID {
		cursor1, cursor2 = cursor1.Next, cursor2.Next
	}
	return cursor1 == nil && cursor2 == nil
}

// isEqualDeclarations reports whether two blocks hold the same declarations
// in the same order.
func isEqualDeclarations(a, b *csstree.List) bool {
	cursor1, cursor2 := a.Head, b.Head
	for cursor1 != nil && cursor2 != nil && declarationID(cursor1.Data) == declarationID(cursor2.Data) {
		cursor1, cursor2 = cursor1.Next, cursor2.Next
	}
	return cursor1 == nil && cursor2 == nil
}

// declarationsDiff is the result of compareDeclarations.
type declarationsDiff struct {
	// eq holds the declarations of the first block also in the second.
	eq []csstree.Node
	// ne1 holds the declarations only in the first block.
	ne1 []csstree.Node
	// ne2 holds the declarations only in the second block that the first
	// block does not override.
	ne2 []csstree.Node
	// ne2overrided holds all declarations only in the second block.
	ne2overrided []csstree.Node
}

// compareDeclarations splits the declarations of two blocks into common and
// distinct ones. It runs after restructBlock, which fails on blocks holding
// anything but declarations.
func compareDeclarations(declarations1, declarations2 *csstree.List) declarationsDiff {
	var result declarationsDiff
	// fingerprint -> important of the first block's declarations
	fingerprints := make(map[string]bool)
	// declarations of the second block not yet matched
	declarations2hash := make(map[int]bool)

	for cursor := declarations2.Head; cursor != nil; cursor = cursor.Next {
		declarations2hash[declarationID(cursor.Data)] = true
	}

	for cursor := declarations1.Head; cursor != nil; cursor = cursor.Next {
		data := cursor.Data.(*csstree.Declaration)
		if data.Fingerprint != "" {
			fingerprints[data.Fingerprint] = data.Important != ""
		}

		if declarations2hash[data.ID] {
			declarations2hash[data.ID] = false
			result.eq = append(result.eq, data)
		} else {
			result.ne1 = append(result.ne1, data)
		}
	}

	for cursor := declarations2.Head; cursor != nil; cursor = cursor.Next {
		data := cursor.Data.(*csstree.Declaration)
		if !declarations2hash[data.ID] {
			continue
		}
		// an overriding declaration in the first block is not a difference,
		// unless only the following one is !important
		important, overridden := fingerprints[data.Fingerprint]
		if !overridden || (!important && data.Important != "") {
			result.ne2 = append(result.ne2, data)
		}
		result.ne2overrided = append(result.ne2overrided, data)
	}

	return result
}

// addSelectors inserts the selectors of source missing from dest, keeping
// dest sorted by selector text, and returns dest.
func addSelectors(dest, source *csstree.List) *csstree.List {
	source.ForEach(func(sourceData csstree.Node, _ *csstree.ListItem) {
		newStr := selector(sourceData).ID
		cursor := dest.Head

		for cursor != nil {
			nextStr := selector(cursor.Data).ID
			if nextStr == newStr {
				return
			}
			if jscompat.Compare(nextStr, newStr) > 0 {
				break
			}
			cursor = cursor.Next
		}

		dest.Insert(csstree.NewListItem(sourceData), cursor)
	})
	return dest
}

// hasSimilarSelectors reports whether any selectors of the two lists have
// equal specificity markers, so the order of their rules matters.
func hasSimilarSelectors(selectors1, selectors2 *csstree.List) bool {
	for cursor1 := selectors1.Head; cursor1 != nil; cursor1 = cursor1.Next {
		for cursor2 := selectors2.Head; cursor2 != nil; cursor2 = cursor2.Next {
			if selector(cursor1.Data).CompareMarker == selector(cursor2.Data).CompareMarker {
				return true
			}
		}
	}
	return false
}

// unsafeToSkipNode reports whether a rule with the given selectors can't be
// moved across node: a rule with similar selectors, or an at-rule without a
// block or with such a rule inside. Declarations can always be skipped.
func unsafeToSkipNode(selectors *csstree.List, node csstree.Node) bool {
	switch n := node.(type) {
	case *csstree.Rule:
		return hasSimilarSelectors(selectorList(n), selectors)
	case *csstree.Atrule:
		if n.Block != nil {
			return n.Block.Children.Some(func(child csstree.Node, _ *csstree.ListItem) bool {
				return unsafeToSkipNode(selectors, child)
			})
		}
	case *csstree.Declaration:
		return false
	}
	// unsafe by default
	return true
}
