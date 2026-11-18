package restructure

import (
	"slices"

	"github.com/css/csso-go/internal/csstree"
)

// atruleGroups collects at-rules by basename and then by name and prelude,
// iterating both levels in insertion order like the JavaScript objects csso
// uses.
type atruleGroups struct {
	names  []string
	groups map[string]*atruleGroup
}

type atruleGroup struct {
	ids   []string
	lists map[string]*csstree.List
}

// add moves the at-rule held by item from list into its group. With single,
// a later at-rule replaces the earlier ones of the same name and prelude.
func (g *atruleGroups) add(item *csstree.ListItem, list *csstree.List, single bool) {
	node := item.Data.(*csstree.Atrule)
	name := csstree.Keyword(node.Name).Basename
	id := csstree.LowerCase(node.Name) + "/" + preludeID(node)

	group, ok := g.groups[name]
	if !ok {
		group = &atruleGroup{lists: make(map[string]*csstree.List)}
		g.groups[name] = group
		g.names = append(g.names, name)
	}

	if single {
		if _, ok := group.lists[id]; ok {
			delete(group.lists, id)
			group.ids = slices.DeleteFunc(group.ids, func(s string) bool { return s == id })
		}
	}

	if _, ok := group.lists[id]; !ok {
		group.lists[id] = csstree.NewList()
		group.ids = append(group.ids, id)
	}

	group.lists[id].Append(list.Remove(item))
}

// preludeID returns the prelude text prepare recorded for an at-rule, or
// "null" (JavaScript's String(null)) when it has no prelude.
func preludeID(node *csstree.Atrule) string {
	switch prelude := node.Prelude.(type) {
	case *csstree.AtrulePrelude:
		return prelude.ID
	case *csstree.Raw:
		return prelude.ID
	}
	return "null"
}

// relocateAtrules moves @keyframes (keeping the last of each name) before
// the first rule, and with forceMediaMerge groups equal @media rules at the
// end of the stylesheet.
func relocateAtrules(ast *csstree.StyleSheet, forceMediaMerge bool) {
	collected := &atruleGroups{groups: make(map[string]*atruleGroup)}
	var topInjectPoint *csstree.ListItem

	ast.Children.ForEach(func(node csstree.Node, item *csstree.ListItem) {
		atrule, ok := node.(*csstree.Atrule)
		if !ok {
			if topInjectPoint == nil {
				topInjectPoint = item
			}
			return
		}

		name := csstree.Keyword(atrule.Name).Basename
		switch name {
		case "keyframes":
			collected.add(item, ast.Children, true)
			return
		case "media":
			if forceMediaMerge {
				collected.add(item, ast.Children, false)
				return
			}
		}

		if topInjectPoint == nil && name != "charset" && name != "import" {
			topInjectPoint = item
		}
	})

	for _, name := range collected.names {
		group := collected.groups[name]
		before := topInjectPoint
		if name == "media" {
			before = nil
		}
		for _, id := range group.ids {
			ast.Children.InsertList(group.lists[id], before)
		}
	}
}

// isMediaRule reports whether node is an @media at-rule.
func isMediaRule(node csstree.Node) bool {
	atrule, ok := node.(*csstree.Atrule)
	return ok && atrule.Name == "media"
}

// mergeAtrule relocates at-rules and merges adjacent @media rules with the
// same query (csso restructure/1-mergeAtrule.js).
func mergeAtrule(ast *csstree.StyleSheet, forceMediaMerge bool) {
	relocateAtrules(ast, forceMediaMerge)

	csstree.Walk(ast, csstree.WalkOptions{
		Visit:   "Atrule",
		Reverse: true,
		Enter: func(_ *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, list *csstree.List) csstree.WalkAction {
			if !isMediaRule(node) || item == nil || item.Prev == nil || !isMediaRule(item.Prev.Data) {
				return csstree.WalkContinue
			}
			current := node.(*csstree.Atrule)
			prev := item.Prev.Data.(*csstree.Atrule)

			// merge @media with the same query
			if current.Prelude != nil && prev.Prelude != nil && preludeID(current) == preludeID(prev) {
				prev.Block.Children.AppendList(current.Block.Children)
				list.Remove(item)
			}
			return csstree.WalkContinue
		},
	})
}
