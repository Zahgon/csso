package defsyntax

// Walk visits node and its descendants depth-first, calling enter before and
// leave after a node's children, as css-tree's definitionSyntax.walk does.
// Either callback may be nil, but not both.
func Walk(node Node, enter, leave func(Node)) {
	if enter == nil && leave == nil {
		panic("defsyntax: neither `enter` nor `leave` walker handler is set")
	}
	walk(node, enter, leave)
}

func walk(node Node, enter, leave func(Node)) {
	if enter != nil {
		enter(node)
	}

	switch n := node.(type) {
	case *Group:
		for _, term := range n.Terms {
			walk(term, enter, leave)
		}
	case *Multiplier:
		walk(n.Term, enter, leave)
	}

	if leave != nil {
		leave(node)
	}
}
