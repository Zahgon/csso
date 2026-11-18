package replace

import (
	"github.com/css/csso-go/internal/csstree"
)

// value shortens the values of font, font-weight, background, border and
// outline declarations (any vendor prefix or hack).
func value(ctx *csstree.WalkContext, node *csstree.Value) {
	if ctx.Declaration == nil {
		return
	}
	switch csstree.Property(ctx.Declaration.Property).Basename {
	case "font":
		font(node)
	case "font-weight":
		fontWeight(node)
	case "background":
		background(node)
	case "border", "outline":
		border(node)
	}
}

// font writes `bold` as 700 and drops `normal` (with a `/` before it); an
// emptied value becomes `normal`.
func font(node *csstree.Value) {
	list := node.Children

	list.ForEachRight(func(data csstree.Node, item *csstree.ListItem) {
		ident, ok := data.(*csstree.Identifier)
		if !ok {
			return
		}
		switch ident.Name {
		case "bold":
			item.Data = &csstree.Number{Base: ident.Base, Value: "700"}
		case "normal":
			if prev := item.Prev; prev != nil {
				if op, ok := prev.Data.(*csstree.Operator); ok && op.Value == "/" {
					list.Remove(prev)
				}
			}
			list.Remove(item)
		}
	})

	if list.IsEmpty() {
		list.AppendData(&csstree.Identifier{Name: "normal"})
	}
}

// fontWeight writes the keywords `normal` and `bold` as 400 and 700.
func fontWeight(node *csstree.Value) {
	head := node.Children.Head
	ident, ok := head.Data.(*csstree.Identifier)
	if !ok {
		return
	}
	switch ident.Name {
	case "normal":
		head.Data = &csstree.Number{Base: ident.Base, Value: "400"}
	case "bold":
		head.Data = &csstree.Number{Base: ident.Base, Value: "700"}
	}
}

// background drops the default values transparent, none, repeat and scroll
// from every layer; a layer left empty becomes `0 0`.
func background(node *csstree.Value) {
	var newValue, buffer []csstree.Node

	flush := func() {
		if len(buffer) == 0 {
			buffer = []csstree.Node{&csstree.Number{Value: "0"}, &csstree.Number{Value: "0"}}
		}
		newValue = append(newValue, buffer...)
		buffer = nil
	}

	for child := range node.Children.All() {
		if op, ok := child.(*csstree.Operator); ok && op.Value == "," {
			flush()
			newValue = append(newValue, child)
			continue
		}

		// remove defaults
		if ident, ok := child.(*csstree.Identifier); ok {
			switch ident.Name {
			case "transparent", "none", "repeat", "scroll":
				continue
			}
		}

		buffer = append(buffer, child)
	}

	flush()
	node.Children = csstree.NewListOf(newValue...)
}

// border drops `none` from border and outline values, or writes it as 0
// when it is the only term.
func border(node *csstree.Value) {
	list := node.Children
	list.ForEach(func(data csstree.Node, item *csstree.ListItem) {
		ident, ok := data.(*csstree.Identifier)
		if !ok || csstree.LowerCase(ident.Name) != "none" {
			return
		}
		if list.Head == list.Tail {
			item.Data = &csstree.Number{Base: ident.Base, Value: "0"}
		} else {
			list.Remove(item)
		}
	})
}
