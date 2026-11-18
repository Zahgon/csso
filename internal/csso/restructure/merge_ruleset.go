package restructure

import (
	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// initialMergeRuleset joins each rule into an earlier one with the same
// pseudo signature and equal selectors or equal declarations, looking back
// across rules and at-rules it may safely skip (csso
// restructure/2-initialMergeRuleset.js). Rules are visited left to right,
// since they merge into the left one.
func initialMergeRuleset(ast *csstree.StyleSheet) {
	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Rule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			selectors := selectorList(rule)
			declarations := rule.Block.Children

			list.PrevUntil(item.Prev, func(prevNode csstree.Node, _ *csstree.ListItem) bool {
				prev, ok := prevNode.(*csstree.Rule)
				// skip non-ruleset node if safe
				if !ok {
					return unsafeToSkipNode(selectors, prevNode)
				}

				prevSelectors := selectorList(prev)
				prevDeclarations := prev.Block.Children

				// try to join rulesets with equal pseudo signature
				if rule.PseudoSignature == prev.PseudoSignature {
					// try to join by selectors
					if isEqualSelectors(prevSelectors, selectors) {
						prevDeclarations.AppendList(declarations)
						list.Remove(item)
						return true
					}

					// try to join by declarations
					if isEqualDeclarations(declarations, prevDeclarations) {
						addSelectors(prevSelectors, selectors)
						list.Remove(item)
						return true
					}
				}

				// go to the previous ruleset if it has no selector similarities
				return hasSimilarSelectors(selectors, prevSelectors)
			})
			return csstree.WalkContinue
		},
	})
}

// disjoinRuleset splits every rule with several selectors into one rule per
// selector (csso restructure/3-disjoinRuleset.js):
//
//	.a, .b { color: red } -> .a { color: red } .b { color: red }
func disjoinRuleset(ast *csstree.StyleSheet) {
	csstree.Walk(ast, csstree.WalkOptions{
		Visit:   "Rule",
		Reverse: true,
		Enter: func(_ *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			selectors := selectorList(rule)

			for selectors.Head != selectors.Tail {
				newSelectors := csstree.NewList()
				newSelectors.Insert(selectors.Remove(selectors.Head), nil)

				list.Insert(csstree.NewListItem(&csstree.Rule{
					Base: rule.Base,
					Prelude: &csstree.SelectorList{
						Base:     csstree.Base{Location: rule.Prelude.Loc()},
						Children: newSelectors,
					},
					Block: &csstree.Block{
						Base:     rule.Block.Base,
						Children: rule.Block.Children.Copy(),
					},
					PseudoSignature: rule.PseudoSignature,
				}), item)
			}
			return csstree.WalkContinue
		},
	})
}

// mergeRuleset joins following rules into each rule, which by now has a
// single selector: rules with the same selector, and rules with equal
// declarations (their selectors are added in sorted order), skipping rules
// whose order doesn't matter (csso restructure/7-mergeRuleset.js):
//
//	.a { color: red } b { ... } .b { color: red } -> .a, .b { color: red } b { ... }
func mergeRuleset(ast *csstree.StyleSheet) {
	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Rule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			selectors := selectorList(rule)
			declarations := rule.Block.Children
			nodeCompareMarker := selector(selectors.First()).CompareMarker
			skippedCompareMarkers := make(map[string]bool)

			list.NextUntil(item.Next, func(nextNode csstree.Node, nextItem *csstree.ListItem) bool {
				next, ok := nextNode.(*csstree.Rule)
				// skip non-ruleset node if safe
				if !ok {
					return unsafeToSkipNode(selectors, nextNode)
				}

				if rule.PseudoSignature != next.PseudoSignature {
					return true
				}

				nextFirstSelector := selectorList(next).Head
				nextDeclarations := next.Block.Children
				nextCompareMarker := selector(nextFirstSelector.Data).CompareMarker

				// stop at a ruleset with the marker of a skipped one
				if skippedCompareMarkers[nextCompareMarker] {
					return true
				}

				// try to join by selectors
				if selectors.Head == selectors.Tail &&
					selector(selectors.First()).ID == selector(nextFirstSelector.Data).ID {
					declarations.AppendList(nextDeclarations)
					list.Remove(nextItem)
					return false
				}

				// try to join by declarations
				if isEqualDeclarations(declarations, nextDeclarations) {
					nextStr := selector(nextFirstSelector.Data).ID

					selectors.Some(func(data csstree.Node, selItem *csstree.ListItem) bool {
						if jscompat.Compare(nextStr, selector(data).ID) < 0 {
							selectors.Insert(nextFirstSelector, selItem)
							return true
						}
						if selItem.Next == nil {
							selectors.Insert(nextFirstSelector, nil)
							return true
						}
						return false
					})

					list.Remove(nextItem)
					return false
				}

				// stop at a ruleset of equal specificity: the order matters
				if nextCompareMarker == nodeCompareMarker {
					return true
				}

				skippedCompareMarkers[nextCompareMarker] = true
				return false
			})
			return csstree.WalkContinue
		},
	})
}
