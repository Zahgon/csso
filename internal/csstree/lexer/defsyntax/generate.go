package defsyntax

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// GenerateOptions controls Generate.
type GenerateOptions struct {
	// ForceBraces wraps every group in square brackets, not only the
	// explicit ones.
	ForceBraces bool
	// Compact omits the optional spaces around combinators and brackets.
	Compact bool
	// Decorate, when set, post-processes the text of every node (and of
	// every multiplier and range suffix) before it is joined into its parent.
	Decorate func(text string, node any) string
}

// Generate serializes a definition syntax tree, as css-tree's
// definitionSyntax.generate does.
func Generate(node Node, opts GenerateOptions) string {
	if opts.Decorate == nil {
		opts.Decorate = func(text string, _ any) string { return text }
	}
	return opts.generate(node)
}

func generateMultiplier(m *Multiplier) string {
	switch {
	case m.Min == 0 && m.Max == 0:
		if m.Comma {
			return "#?"
		}
		return "*"
	case m.Min == 0 && m.Max == 1:
		return "?"
	case m.Min == 1 && m.Max == 0:
		if m.Comma {
			return "#"
		}
		return "+"
	case m.Min == 1 && m.Max == 1:
		return ""
	}

	prefix := ""
	if m.Comma {
		prefix = "#"
	}
	if m.Min == m.Max {
		return prefix + "{" + strconv.Itoa(m.Min) + "}"
	}
	maxText := ""
	if m.Max != 0 {
		maxText = strconv.Itoa(m.Max)
	}
	return prefix + "{" + strconv.Itoa(m.Min) + "," + maxText + "}"
}

// String formats the bound as JavaScript prints css-tree's number (or
// number-plus-unit string) bound.
func (b *RangeBound) String() string {
	return formatNumber(b.Value) + b.Unit
}

// formatNumber prints a float like JavaScript's Number#toString for the
// integral values range bounds hold.
func formatNumber(v float64) string {
	switch {
	case v == 0:
		return "0" // also for -0, as JavaScript does
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case math.Abs(v) >= 1e21:
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func generateTypeOpts(r *Range) string {
	minText, maxText := "-∞", "∞"
	if r.Min != nil {
		minText = r.Min.String()
	}
	if r.Max != nil {
		maxText = r.Max.String()
	}
	return " [" + minText + "," + maxText + "]"
}

func (o GenerateOptions) generateSequence(g *Group) string {
	combinator := g.Combinator
	if combinator != " " && !o.Compact {
		combinator = " " + combinator + " "
	}

	parts := make([]string, len(g.Terms))
	for i, term := range g.Terms {
		parts[i] = o.generate(term)
	}
	result := strings.Join(parts, combinator)

	if g.Explicit || o.ForceBraces {
		open, closing := "[ ", " ]"
		if o.Compact || strings.HasPrefix(result, ",") {
			open = "["
		}
		if o.Compact {
			closing = "]"
		}
		return open + result + closing
	}
	return result
}

func (o GenerateOptions) generate(node Node) string {
	var result string

	switch n := node.(type) {
	case *Group:
		result = o.generateSequence(n)
		if n.DisallowEmpty {
			result += "!"
		}
	case *Multiplier:
		// a multiplier is a composition: its term is decorated on its own
		return o.generate(n.Term) + o.Decorate(generateMultiplier(n), n)
	case *Type:
		result = "<" + n.Name
		if n.Opts != nil {
			result += o.Decorate(generateTypeOpts(n.Opts), n.Opts)
		}
		result += ">"
	case *Property:
		result = "<'" + n.Name + "'>"
	case *Keyword:
		result = n.Name
	case *AtKeyword:
		result = "@" + n.Name
	case *Function:
		result = n.Name + "("
	case *String:
		result = n.Value
	case *Token:
		result = n.Value
	case *Comma:
		result = ","
	default:
		panic(fmt.Sprintf("defsyntax: unknown node type `%T`", node))
	}

	return o.Decorate(result, node)
}
