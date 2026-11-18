package csstree

import (
	"strconv"
	"strings"
)

// JSON serializes the tree rooted at node as JSON.stringify does with the
// node objects css-tree's parser creates: the same keys in the same order,
// `loc` as {source, start, end} or null, lists as arrays and absent optional
// values as null. The annotations csso adds while compressing are not part
// of the output.
func JSON(node Node) string {
	var b strings.Builder
	writeJSONNode(&b, node)
	return b.String()
}

// jsonObject writes the members of one JSON object.
type jsonObject struct {
	b     *strings.Builder
	first bool
}

func (o *jsonObject) key(name string) {
	if !o.first {
		o.b.WriteByte(',')
	}
	o.first = false
	writeJSONString(o.b, name)
	o.b.WriteByte(':')
}

func (o *jsonObject) str(name, value string) {
	o.key(name)
	writeJSONString(o.b, value)
}

// optStr writes value, or null when it is empty (css-tree's null).
func (o *jsonObject) optStr(name, value string) {
	if value == "" {
		o.key(name)
		o.b.WriteString("null")
		return
	}
	o.str(name, value)
}

func (o *jsonObject) node(name string, value Node) {
	o.key(name)
	writeJSONNode(o.b, value)
}

func (o *jsonObject) list(name string, value *List) {
	o.key(name)
	if value == nil {
		o.b.WriteString("null")
		return
	}
	o.b.WriteByte('[')
	for item := value.Head; item != nil; item = item.Next {
		if item != value.Head {
			o.b.WriteByte(',')
		}
		writeJSONNode(o.b, item.Data)
	}
	o.b.WriteByte(']')
}

func writeJSONNode(b *strings.Builder, node Node) {
	if node == nil {
		b.WriteString("null")
		return
	}

	b.WriteByte('{')
	o := &jsonObject{b: b, first: true}
	o.str("type", node.Type())
	o.key("loc")
	writeJSONLocation(b, node.Loc())

	switch n := node.(type) {
	case *Atrule:
		o.str("name", n.Name)
		o.node("prelude", n.Prelude)
		o.node("block", blockField(n.Block))
	case *Rule:
		o.node("prelude", n.Prelude)
		o.node("block", blockField(n.Block))
	case *Declaration:
		o.key("important")
		switch n.Important {
		case "":
			b.WriteString("false")
		case "important":
			b.WriteString("true")
		default:
			writeJSONString(b, n.Important)
		}
		o.str("property", n.Property)
		o.node("value", n.Value)
	case *Identifier:
		o.str("name", n.Name)
	case *TypeSelector:
		o.str("name", n.Name)
	case *IdSelector:
		o.str("name", n.Name)
	case *ClassSelector:
		o.str("name", n.Name)
	case *Combinator:
		o.str("name", n.Name)
	case *Number:
		o.str("value", n.Value)
	case *Percentage:
		o.str("value", n.Value)
	case *String:
		o.str("value", n.Value)
	case *Url:
		o.str("value", n.Value)
	case *Hash:
		o.str("value", n.Value)
	case *Operator:
		o.str("value", n.Value)
	case *Raw:
		o.str("value", n.Value)
	case *Comment:
		o.str("value", n.Value)
	case *WhiteSpace:
		o.str("value", n.Value)
	case *UnicodeRange:
		o.str("value", n.Value)
	case *Dimension:
		o.str("value", n.Value)
		o.str("unit", n.Unit)
	case *Function:
		o.str("name", n.Name)
		o.list("children", n.Children)
	case *PseudoClassSelector:
		o.str("name", n.Name)
		o.list("children", n.Children)
	case *PseudoElementSelector:
		o.str("name", n.Name)
		o.list("children", n.Children)
	case *AttributeSelector:
		var name Node
		if n.Name != nil {
			name = n.Name
		}
		o.node("name", name)
		o.optStr("matcher", n.Matcher)
		o.node("value", n.Value)
		o.optStr("flags", n.Flags)
	case *Nth:
		var selector Node
		if n.Selector != nil {
			selector = n.Selector
		}
		o.node("nth", n.Nth)
		o.node("selector", selector)
	case *AnPlusB:
		o.optStr("a", n.A)
		o.optStr("b", n.B)
	case *MediaFeature:
		o.str("name", n.Name)
		o.node("value", n.Value)
	case *Ratio:
		o.str("left", n.Left)
		o.str("right", n.Right)
	case *CDO, *CDC:
	default:
		if children := Children(node); children != nil {
			o.list("children", children)
		}
	}
	b.WriteByte('}')
}

func writeJSONLocation(b *strings.Builder, loc *Location) {
	if loc == nil {
		b.WriteString("null")
		return
	}
	b.WriteString(`{"source":`)
	writeJSONString(b, loc.Source)
	b.WriteString(`,"start":`)
	writeJSONPosition(b, loc.Start)
	b.WriteString(`,"end":`)
	writeJSONPosition(b, loc.End)
	b.WriteByte('}')
}

func writeJSONPosition(b *strings.Builder, p Position) {
	b.WriteString(`{"offset":`)
	b.WriteString(strconv.Itoa(p.Offset))
	b.WriteString(`,"line":`)
	b.WriteString(strconv.Itoa(p.Line))
	b.WriteString(`,"column":`)
	b.WriteString(strconv.Itoa(p.Column))
	b.WriteByte('}')
}
