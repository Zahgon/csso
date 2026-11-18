package csstree

import "fmt"

// Clone returns a deep copy of the tree rooted at node, like css-tree's
// clone(): child nodes, lists and locations are copied, other fields
// (including csso's annotations) are copied by value.
func Clone(node Node) Node {
	if node == nil {
		return nil
	}

	var c Node
	switch n := node.(type) {
	case *StyleSheet:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Atrule:
		cp := *n
		cp.Prelude = Clone(n.Prelude)
		cp.Block = cloneBlock(n.Block)
		c = &cp
	case *AtrulePrelude:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Rule:
		cp := *n
		cp.Prelude = Clone(n.Prelude)
		cp.Block = cloneBlock(n.Block)
		c = &cp
	case *SelectorList:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Selector:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Block:
		c = cloneBlock(n)
	case *Declaration:
		cp := *n
		cp.Value = Clone(n.Value)
		c = &cp
	case *DeclarationList:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Value:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Function:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Parentheses:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *Brackets:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *PseudoClassSelector:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *PseudoElementSelector:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *MediaQueryList:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *MediaQuery:
		cp := *n
		cp.Children = cloneList(n.Children)
		c = &cp
	case *AttributeSelector:
		cp := *n
		if n.Name != nil {
			cp.Name = Clone(n.Name).(*Identifier)
		}
		cp.Value = Clone(n.Value)
		c = &cp
	case *MediaFeature:
		cp := *n
		cp.Value = Clone(n.Value)
		c = &cp
	case *Nth:
		cp := *n
		cp.Nth = Clone(n.Nth)
		if n.Selector != nil {
			cp.Selector = Clone(n.Selector).(*SelectorList)
		}
		c = &cp
	case *Identifier:
		cp := *n
		c = &cp
	case *Number:
		cp := *n
		c = &cp
	case *Dimension:
		cp := *n
		c = &cp
	case *Percentage:
		cp := *n
		c = &cp
	case *String:
		cp := *n
		c = &cp
	case *Url:
		cp := *n
		c = &cp
	case *Hash:
		cp := *n
		c = &cp
	case *Operator:
		cp := *n
		c = &cp
	case *Raw:
		cp := *n
		c = &cp
	case *Comment:
		cp := *n
		c = &cp
	case *WhiteSpace:
		cp := *n
		c = &cp
	case *CDO:
		cp := *n
		c = &cp
	case *CDC:
		cp := *n
		c = &cp
	case *TypeSelector:
		cp := *n
		c = &cp
	case *IdSelector:
		cp := *n
		c = &cp
	case *ClassSelector:
		cp := *n
		c = &cp
	case *Combinator:
		cp := *n
		c = &cp
	case *AnPlusB:
		cp := *n
		c = &cp
	case *Ratio:
		cp := *n
		c = &cp
	case *UnicodeRange:
		cp := *n
		c = &cp
	default:
		panic(fmt.Sprintf("csstree: unknown node type %T", node))
	}

	if loc := node.Loc(); loc != nil {
		cp := *loc
		c.SetLoc(&cp)
	}
	return c
}

func cloneBlock(b *Block) *Block {
	if b == nil {
		return nil
	}
	cp := *b
	cp.Children = cloneList(b.Children)
	if b.Location != nil {
		loc := *b.Location
		cp.Location = &loc
	}
	return &cp
}

// cloneList returns a new list holding clones of the nodes of list (nil
// for a nil list).
func cloneList(list *List) *List {
	if list == nil {
		return nil
	}
	result := NewList()
	for it := list.Head; it != nil; it = it.Next {
		result.AppendData(Clone(it.Data))
	}
	return result
}
