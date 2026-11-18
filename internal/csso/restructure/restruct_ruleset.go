package restructure

import (
	"github.com/css/csso-go/internal/csstree"
)

// calcSelectorLength returns the length of the selectors joined by commas,
// in UTF-16 code units.
func calcSelectorLength(list *csstree.List) int {
	length := 0
	for data := range list.All() {
		length += csstree.UTF16Len(selector(data).ID) + 1
	}
	return length - 1
}

// calcDeclarationsLength returns the length of the declarations joined by
// semicolons, in UTF-16 code units.
func calcDeclarationsLength(tokens []csstree.Node) int {
	length := 0
	for _, token := range tokens {
		length += token.(*csstree.Declaration).Length
	}
	return length + len(tokens) - 1
}

// restructRuleset moves declarations between rules looking back from each
// rule (csso restructure/8-restructRuleset.js): equal blocks are merged, a
// subset block takes the other rule's selectors when that is shorter, and
// declarations shared by two rules are extracted into a new rule when that
// saves space. Rules with similar selectors and at-rules containing them
// limit how far rules may move up or down.
func restructRuleset(ast *csstree.StyleSheet) {
	csstree.Walk(ast, csstree.WalkOptions{
		Visit:   "Rule",
		Reverse: true,
		Enter: func(ctx *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			processRuleset(ctx, node.(*csstree.Rule), item, list)
			return csstree.WalkContinue
		},
	})
}

func processRuleset(ctx *csstree.WalkContext, node *csstree.Rule, item *csstree.ListItem, list *csstree.List) {
	avoidRulesMerge := ctx.Block != nil && ctx.Block.AvoidRulesMerge
	selectors := selectorList(node)
	block := node.Block
	disallowDownMarkers := make(map[string]bool)
	allowMergeUp := true
	allowMergeDown := true

	list.PrevUntil(item.Prev, func(prevNode csstree.Node, prevItem *csstree.ListItem) bool {
		prev, ok := prevNode.(*csstree.Rule)
		if !ok {
			unsafe := unsafeToSkipNode(selectors, prevNode)

			if atrule, ok := prevNode.(*csstree.Atrule); ok && !unsafe && atrule.Block != nil {
				csstree.Walk(atrule.Block, csstree.WalkOptions{
					Visit: "Rule",
					Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
						for data := range selectorList(node.(*csstree.Rule)).All() {
							disallowDownMarkers[selector(data).CompareMarker] = true
						}
						return csstree.WalkContinue
					},
				})
			}

			return unsafe
		}

		if node.PseudoSignature != prev.PseudoSignature {
			return true
		}

		prevSelectors := selectorList(prev)
		prevBlock := prev.Block

		allowMergeDown = !prevSelectors.Some(func(data csstree.Node, _ *csstree.ListItem) bool {
			return disallowDownMarkers[selector(data).CompareMarker]
		})

		// try the previous ruleset only if selectors have no equal
		// specificity nor element selector
		if !allowMergeDown && !allowMergeUp {
			return true
		}

		// try to join by selectors
		if allowMergeUp && isEqualSelectors(prevSelectors, selectors) {
			prevBlock.Children.AppendList(block.Children)
			list.Remove(item)
			return true
		}

		// try to join by properties
		diff := compareDeclarations(block.Children, prevBlock.Children)

		if len(diff.eq) > 0 {
			switch {
			case len(diff.ne1) == 0 && len(diff.ne2) == 0:
				// equal blocks
				if allowMergeDown {
					addSelectors(selectors, prevSelectors)
					list.Remove(prevItem)
				}
				return true

			case avoidRulesMerge:
				// keyframe rules are only merged when equal

			case len(diff.ne1) > 0 && len(diff.ne2) == 0:
				// prevBlock is a subset of block
				selectorLength := calcSelectorLength(selectors)
				blockLength := calcDeclarationsLength(diff.eq)

				if allowMergeUp && selectorLength < blockLength {
					addSelectors(prevSelectors, selectors)
					block.Children.FromSlice(diff.ne1)
				}

			case len(diff.ne1) == 0 && len(diff.ne2) > 0:
				// block is a subset of prevBlock
				selectorLength := calcSelectorLength(prevSelectors)
				blockLength := calcDeclarationsLength(diff.eq)

				if allowMergeDown && selectorLength < blockLength {
					addSelectors(selectors, prevSelectors)
					prevBlock.Children.FromSlice(diff.ne2)
				}

			default:
				// extract the equal declarations into a new ruleset when they
				// are longer than the ruleset overhead
				newSelector := &csstree.SelectorList{Children: addSelectors(prevSelectors.Copy(), selectors)}
				newBlockLength := calcSelectorLength(newSelector.Children) + 2 // selectors and braces
				blockLength := calcDeclarationsLength(diff.eq)

				if blockLength >= newBlockLength {
					newItem := csstree.NewListItem(&csstree.Rule{
						Prelude: newSelector,
						Block: &csstree.Block{
							Children: csstree.NewListOf(diff.eq...),
						},
						PseudoSignature: node.PseudoSignature,
					})

					block.Children.FromSlice(diff.ne1)
					prevBlock.Children.FromSlice(diff.ne2overrided)

					if allowMergeUp {
						list.Insert(newItem, prevItem)
					} else {
						list.Insert(newItem, item)
					}
					return true
				}
			}
		}

		if allowMergeUp {
			// disallow merging up past rules with similar selectors
			allowMergeUp = !prevSelectors.Some(func(prevSelector csstree.Node, _ *csstree.ListItem) bool {
				return selectors.Some(func(sel csstree.Node, _ *csstree.ListItem) bool {
					return selector(sel).CompareMarker == selector(prevSelector).CompareMarker
				})
			})
		}

		for data := range prevSelectors.All() {
			disallowDownMarkers[selector(data).CompareMarker] = true
		}
		return false
	})
}
