package restructure

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csso/nodes"
	"github.com/css/csso-go/internal/csso/usage"
	"github.com/css/csso-go/internal/csstree"
)

// declarationIndexer gives equal declarations (by generated text) equal
// ids (csso restructure/prepare/createDeclarationIndexer.js).
type declarationIndexer struct {
	ids map[string]int
}

func newDeclarationIndexer() *declarationIndexer {
	return &declarationIndexer{ids: make(map[string]int)}
}

// mark annotates a declaration with its id and generated length and resets
// its fingerprint.
func (x *declarationIndexer) mark(node *csstree.Declaration) *csstree.Declaration {
	text := csstree.Generate(node)
	id, ok := x.ids[text]
	if !ok {
		id = len(x.ids) + 1
		x.ids[text] = id
	}
	node.ID = id
	node.Length = csstree.UTF16Len(text)
	node.Fingerprint = ""
	return node
}

// prepare annotates the tree for restructuring: declaration ids, selector
// ids and specificity markers, pseudo signatures of rules and at-rule
// prelude ids.
func prepare(ast *csstree.StyleSheet, u *usage.Index) (*declarationIndexer, error) {
	indexer := newDeclarationIndexer()
	var err error

	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Rule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			rule := node.(*csstree.Rule)
			for child := range rule.Block.Children.All() {
				if decl, ok := child.(*csstree.Declaration); ok {
					indexer.mark(decl)
				}
			}
			if err = processSelector(rule, u); err != nil {
				return csstree.WalkBreak
			}
			return csstree.WalkContinue
		},
	})
	if err != nil {
		return nil, err
	}

	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Atrule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			atrule := node.(*csstree.Atrule)
			switch prelude := atrule.Prelude.(type) {
			case *csstree.AtrulePrelude:
				prelude.ID = csstree.Generate(prelude)
			case *csstree.Raw:
				prelude.ID = csstree.Generate(prelude)
			}

			// compare keyframe selectors by their text (csso issue #197)
			if csstree.Keyword(atrule.Name).Basename == "keyframes" && atrule.Block != nil {
				atrule.Block.AvoidRulesMerge = true
				for child := range atrule.Block.Children.All() {
					rule, ok := child.(*csstree.Rule)
					if !ok {
						continue
					}
					for s := range selectorList(rule).All() {
						if sel, ok := s.(*csstree.Selector); ok {
							sel.CompareMarker = sel.ID
						}
					}
				}
			}
			return csstree.WalkContinue
		},
	})

	return indexer, nil
}

// nonFreezePseudoElements and nonFreezePseudoClasses are the pseudos that
// don't stop rules from being merged.
var (
	nonFreezePseudoElements = map[string]bool{
		"first-letter": true, "first-line": true, "after": true, "before": true,
	}
	nonFreezePseudoClasses = map[string]bool{
		"link": true, "visited": true, "hover": true, "active": true,
		"first-letter": true, "first-line": true, "after": true, "before": true,
	}
)

// processSelector annotates the selectors of a rule with their text (ID)
// and a compare marker made of the specificity, the usage scope and the
// element name, and the rule with the sorted list of its freezing pseudo
// classes and elements (csso restructure/prepare/processSelector.js).
func processSelector(rule *csstree.Rule, u *usage.Index) error {
	pseudos := make(map[string]bool)

	selectors, err := nodes.ChildList(rule.Prelude, "forEach")
	if err != nil {
		return err
	}
	for item := selectors.Head; item != nil; item = item.Next {
		simpleSelector, ok := item.Data.(*csstree.Selector)
		if !ok {
			continue
		}
		tagName := "*"
		scope := 0

		for child := range simpleSelector.Children.All() {
			switch n := child.(type) {
			case *csstree.ClassSelector:
				if u != nil && u.Scopes != nil {
					classScope := u.Scopes[n.Name]
					if scope != 0 && classScope != scope {
						return jscompat.NewError("Selector can't has classes from different scopes: " + csstree.Generate(simpleSelector))
					}
					scope = classScope
				}
			case *csstree.PseudoClassSelector:
				if name := csstree.LowerCase(n.Name); !nonFreezePseudoClasses[name] {
					pseudos[":"+name] = true
				}
			case *csstree.PseudoElementSelector:
				if name := csstree.LowerCase(n.Name); !nonFreezePseudoElements[name] {
					pseudos["::"+name] = true
				}
			case *csstree.TypeSelector:
				tagName = csstree.LowerCase(n.Name)
			case *csstree.AttributeSelector:
				if n.Flags != "" {
					pseudos["["+csstree.LowerCase(n.Flags)+"]"] = true
				}
			case *csstree.Combinator:
				tagName = "*"
			}
		}

		spec, err := specificity(simpleSelector)
		if err != nil {
			return err
		}
		simpleSelector.CompareMarker = fmt.Sprintf("%d,%d,%d", spec[0], spec[1], spec[2])
		simpleSelector.ID = csstree.Generate(simpleSelector)

		if scope != 0 {
			simpleSelector.CompareMarker += ":" + strconv.Itoa(scope)
		}
		if tagName != "*" {
			simpleSelector.CompareMarker += "," + tagName
		}
	}

	signature := make([]string, 0, len(pseudos))
	for pseudo := range pseudos {
		signature = append(signature, pseudo)
	}
	slices.SortFunc(signature, jscompat.Compare)
	rule.PseudoSignature = strings.Join(signature, ",")
	return nil
}

// Specificity is a selector specificity (A, B, C).
type Specificity [3]int

// maxSpecificity returns the greater of two specificities.
func maxSpecificity(a, b Specificity) Specificity {
	for i := range a {
		if a[i] != b[i] {
			if a[i] > b[i] {
				return a
			}
			return b
		}
	}
	return a
}

// maxSelectorListSpecificity returns the specificity of the most specific
// selector of a selector list argument; an unparsed (Raw) argument is parsed
// as a selector list first.
func maxSelectorListSpecificity(selectorList csstree.Node) (Specificity, error) {
	switch n := selectorList.(type) {
	case nil:
		return Specificity{}, jscompat.ReadPropertyError("null", "type")
	case *csstree.Raw:
		parsed, err := csstree.Parse(n.Value, &csstree.ParseOptions{Context: csstree.ContextSelectorList})
		if err != nil {
			return Specificity{}, err
		}
		selectorList = parsed
	}

	children, err := nodes.ChildList(selectorList, "reduce")
	if err != nil {
		return Specificity{}, err
	}
	var result Specificity
	for child := range children.All() {
		spec, err := specificity(child)
		if err != nil {
			return Specificity{}, err
		}
		result = maxSpecificity(spec, result)
	}
	return result, nil
}

// firstArgument returns the first child of a pseudo-class selector, failing
// like csso's `node.children.first` for a pseudo-class without arguments.
func firstArgument(node *csstree.PseudoClassSelector) (csstree.Node, error) {
	if node.Children == nil {
		return nil, jscompat.ReadPropertyError("null", "first")
	}
	return node.Children.First(), nil
}

// specificity computes the specificity of a complex selector as defined by
// Selectors Level 4, §16 (csso restructure/prepare/specificity.js).
func specificity(simpleSelector csstree.Node) (Specificity, error) {
	var a, b, c int

	children, err := nodes.ChildList(simpleSelector, "forEach")
	if err != nil {
		return Specificity{}, err
	}
	for child := range children.All() {
		switch n := child.(type) {
		// count the number of ID selectors (= A)
		case *csstree.IdSelector:
			a++

		// count the number of class selectors, attribute selectors...
		case *csstree.ClassSelector, *csstree.AttributeSelector:
			b++

		// ... and pseudo-classes (= B)
		case *csstree.PseudoClassSelector:
			switch csstree.LowerCase(n.Name) {
			// :is(), :not() and :has() (and their predecessors :matches(),
			// :-webkit-any() and :-moz-any()) count as their most specific
			// argument
			case "not", "has", "is", "matches", "-webkit-any", "-moz-any":
				arg, err := firstArgument(n)
				if err != nil {
					return Specificity{}, err
				}
				spec, err := maxSelectorListSpecificity(arg)
				if err != nil {
					return Specificity{}, err
				}
				a += spec[0]
				b += spec[1]
				c += spec[2]

			// :nth-child() and :nth-last-child() count as a pseudo-class
			// plus their most specific `of` selector
			case "nth-child", "nth-last-child":
				arg, err := firstArgument(n)
				if err != nil {
					return Specificity{}, err
				}
				if arg == nil {
					return Specificity{}, jscompat.ReadPropertyError("null", "type")
				}
				if nth, ok := arg.(*csstree.Nth); ok && nth.Selector != nil {
					spec, err := maxSelectorListSpecificity(nth.Selector)
					if err != nil {
						return Specificity{}, err
					}
					a += spec[0]
					b += spec[1] + 1
					c += spec[2]
				} else {
					b++
				}

			// :where() counts as zero
			case "where":

			// legacy single-colon pseudo-elements
			case "before", "after", "first-line", "first-letter":
				c++

			default:
				b++
			}

		// count the number of type selectors, ignoring the universal
		// selector...
		case *csstree.TypeSelector:
			if !strings.HasSuffix(n.Name, "*") {
				c++
			}

		// ... and pseudo-elements (= C)
		case *csstree.PseudoElementSelector:
			c++
		}
	}

	return Specificity{a, b, c}, nil
}
