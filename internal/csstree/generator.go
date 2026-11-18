package csstree

import (
	"fmt"
	"strings"
)

// This file ports css-tree's lib/generator (create.js, token-before.js) and
// the generate() functions of lib/syntax/node/*.js, with csso's overrides
// for String and Url nodes (csso lib/syntax.js).

// GeneratedToken is a token emitted while generating CSS. Node is the node
// that emitted it, or nil for whitespace the generator inserted on its own.
type GeneratedToken struct {
	Type  TokenType
	Value string
	Node  Node
}

// Generate serializes the tree rooted at node into CSS. Whitespace is only
// inserted between tokens that would otherwise merge (css-tree's "safe"
// mode).
func Generate(node Node) string {
	g := &generator{}
	g.node(node)
	return g.buf.String()
}

// GenerateWithSourceMap serializes node like Generate and also builds a
// source map from the locations of Atrule, Selector and Declaration nodes.
func GenerateWithSourceMap(node Node) (string, *SourceMap) {
	g := &generator{sourceMap: newSourceMapTracker()}
	g.node(node)
	return g.buf.String(), g.sourceMap.finish()
}

// GenerateTokens returns the tokens Generate would emit for node, each with
// the node that emitted it (css-tree's generate with a token collecting
// decorator, as used by its lexer).
// String and Url nodes always emit exactly one token attributed to them
// (whitespace inserted before it is attributed to no node); the lexer relies
// on this to match such nodes as a whole.
func GenerateTokens(node Node) []GeneratedToken {
	g := &generator{collectTokens: true}
	g.node(node)
	return g.tokens
}

type generator struct {
	buf      strings.Builder
	prevCode int

	sourceMap *sourceMapTracker

	collectTokens bool
	tokens        []GeneratedToken
	currentNode   Node
}

func (g *generator) emit(value string, t TokenType, auto bool) {
	if g.sourceMap != nil {
		g.sourceMap.advance(value)
	}
	if g.collectTokens {
		var owner Node
		if !auto {
			owner = g.currentNode
		}
		g.tokens = append(g.tokens, GeneratedToken{Type: t, Value: value, Node: owner})
		return
	}
	g.buf.WriteString(value)
}

// token emits a token, preceded by a space when it would merge with the
// previous one. A delim starting with a reverse solidus is followed by a
// newline so it cannot form an escape with the next token.
func (g *generator) token(t TokenType, value string) {
	g.prevCode = g.tokenBefore(g.prevCode, t, value)
	g.emit(value, t, false)
	if t == DelimToken && firstUnit(value) == '\\' {
		g.emit("\n", WhiteSpaceToken, true)
	}
}

// tokenize emits chunk token by token.
func (g *generator) tokenize(chunk string) {
	units := toUnits(chunk)
	tokenizeUnits(units, func(t TokenType, start, end int) {
		g.token(t, sliceUnits(units, start, end))
	})
}

// children generates every node of list, calling between (when set) with
// the previous node before each node but the first.
func (g *generator) children(list *List, between func(prev Node)) {
	var prev Node
	list.ForEach(func(child Node, _ *ListItem) {
		if prev != nil && between != nil {
			between(prev)
		}
		g.node(child)
		prev = child
	})
}

func (g *generator) node(n Node) {
	if g.sourceMap != nil {
		g.sourceMap.enterNode(n)
	}
	if g.collectTokens {
		saved := g.currentNode
		g.currentNode = n
		defer func() { g.currentNode = saved }()
	}

	g.generateNode(n)

	if g.sourceMap != nil {
		g.sourceMap.leaveNode(n)
	}
}

func (g *generator) generateNode(node Node) {
	switch n := node.(type) {
	case *AnPlusB:
		g.generateAnPlusB(n)
	case *Atrule:
		g.token(AtKeywordToken, "@"+n.Name)
		if n.Prelude != nil {
			g.node(n.Prelude)
		}
		if n.Block != nil {
			g.node(n.Block)
		} else {
			g.token(SemicolonToken, ";")
		}
	case *AtrulePrelude:
		g.children(n.Children, nil)
	case *AttributeSelector:
		g.token(DelimToken, "[")
		g.node(n.Name)
		if n.Matcher != "" {
			g.tokenize(n.Matcher)
			g.node(n.Value)
		}
		if n.Flags != "" {
			g.token(IdentToken, n.Flags)
		}
		g.token(DelimToken, "]")
	case *Block:
		g.token(LeftCurlyBracketToken, "{")
		g.children(n.Children, g.semicolonAfterDeclaration)
		g.token(RightCurlyBracketToken, "}")
	case *Brackets:
		g.token(DelimToken, "[")
		g.children(n.Children, nil)
		g.token(DelimToken, "]")
	case *CDC:
		g.token(CDCToken, "-->")
	case *CDO:
		g.token(CDOToken, "<!--")
	case *ClassSelector:
		g.token(DelimToken, ".")
		g.token(IdentToken, n.Name)
	case *Combinator:
		g.tokenize(n.Name)
	case *Comment:
		g.token(CommentToken, "/*"+n.Value+"*/")
	case *Declaration:
		g.token(IdentToken, n.Property)
		g.token(ColonToken, ":")
		g.node(n.Value)
		if n.Important != "" {
			g.token(DelimToken, "!")
			g.token(IdentToken, n.Important)
		}
	case *DeclarationList:
		g.children(n.Children, g.semicolonAfterDeclaration)
	case *Dimension:
		g.token(DimensionToken, n.Value+n.Unit)
	case *Function:
		g.token(FunctionToken, n.Name+"(")
		g.children(n.Children, nil)
		g.token(RightParenthesisToken, ")")
	case *Hash:
		g.token(HashToken, "#"+n.Value)
	case *Identifier:
		g.token(IdentToken, n.Name)
	case *IdSelector:
		// css-tree emits a delim rather than a hash so no space is required
		// after an identifier (e.g. "a#id")
		g.token(DelimToken, "#"+n.Name)
	case *MediaFeature:
		g.token(LeftParenthesisToken, "(")
		g.token(IdentToken, n.Name)
		if n.Value != nil {
			g.token(ColonToken, ":")
			g.node(n.Value)
		}
		g.token(RightParenthesisToken, ")")
	case *MediaQuery:
		g.children(n.Children, nil)
	case *MediaQueryList:
		g.children(n.Children, g.comma)
	case *Nth:
		g.node(n.Nth)
		if n.Selector != nil {
			g.token(IdentToken, "of")
			g.node(n.Selector)
		}
	case *Number:
		g.token(NumberToken, n.Value)
	case *Operator:
		g.tokenize(n.Value)
	case *Parentheses:
		g.token(LeftParenthesisToken, "(")
		g.children(n.Children, nil)
		g.token(RightParenthesisToken, ")")
	case *Percentage:
		g.token(PercentageToken, n.Value+"%")
	case *PseudoClassSelector:
		g.token(ColonToken, ":")
		g.generatePseudo(n.Name, n.Children)
	case *PseudoElementSelector:
		g.token(ColonToken, ":")
		g.token(ColonToken, ":")
		g.generatePseudo(n.Name, n.Children)
	case *Ratio:
		g.token(NumberToken, n.Left)
		g.token(DelimToken, "/")
		g.token(NumberToken, n.Right)
	case *Raw:
		g.tokenize(n.Value)
	case *Rule:
		g.node(n.Prelude)
		g.node(n.Block)
	case *Selector:
		g.children(n.Children, nil)
	case *SelectorList:
		g.children(n.Children, g.comma)
	case *String:
		g.token(StringToken, shortestQuotedString(n.Value))
	case *StyleSheet:
		g.children(n.Children, nil)
	case *TypeSelector:
		g.tokenize(n.Name)
	case *UnicodeRange:
		g.tokenize(n.Value)
	case *Url:
		g.generateURL(n)
	case *Value:
		g.children(n.Children, nil)
	case *WhiteSpace:
		g.token(WhiteSpaceToken, n.Value)
	default:
		panic(fmt.Sprintf("csstree: unknown node type %T", node))
	}
}

func (g *generator) semicolonAfterDeclaration(prev Node) {
	if prev.Type() == "Declaration" {
		g.token(SemicolonToken, ";")
	}
}

func (g *generator) comma(Node) {
	g.token(CommaToken, ",")
}

func (g *generator) generatePseudo(name string, children *List) {
	if children == nil {
		g.token(IdentToken, name)
		return
	}
	g.token(FunctionToken, name+"(")
	g.children(children, nil)
	g.token(RightParenthesisToken, ")")
}

// generateAnPlusB emits the shortest An+B form: "n" for a coefficient of 1,
// "-n" for -1, and an explicit sign before B.
func (g *generator) generateAnPlusB(n *AnPlusB) {
	if n.A == "" {
		g.tokenize(n.B)
		return
	}

	var a string
	switch n.A {
	case "+1", "1":
		a = "n"
	case "-1":
		a = "-n"
	default:
		a = n.A + "n"
	}

	if n.B == "" {
		g.tokenize(a)
		return
	}
	b := n.B
	if b[0] != '-' && b[0] != '+' {
		b = "+" + b
	}
	g.tokenize(a + b)
}

// shortestQuotedString is csso's encodeString: the shorter of the single-
// and double-quoted serializations, preferring double quotes on a tie.
func shortestQuotedString(value string) string {
	apostrophe := EncodeString(value, true)
	quote := EncodeString(value, false)
	if UTF16Len(apostrophe) < UTF16Len(quote) {
		return apostrophe
	}
	return quote
}

// generateURL is csso's Url generator: an unquoted url() unless the quoted
// form is more than five characters shorter.
func (g *generator) generateURL(n *Url) {
	encoded := EncodeURL(n.Value)
	quoted := shortestQuotedString(n.Value)
	if UTF16Len(encoded) <= UTF16Len(quoted)+len("url()") {
		g.token(URLToken, encoded)
		return
	}
	g.token(URLToken, "url("+quoted+")")
}

// whiteSpaceRequired holds the token pairs css-tree's safe mode separates
// with whitespace, encoded with pairCode.
var whiteSpaceRequired = buildWhiteSpacePairs()

// tokenCode identifies a token for whitespace decisions: delims by their
// character (shifted past the token type range, non-ASCII collapsed),
// everything else by type.
func tokenCode(t TokenType, value string) int {
	if t != DelimToken {
		return int(t)
	}
	return charCode(firstUnit(value))
}

func charCode(c int) int {
	if c < 0 {
		return 0
	}
	if c > 0x7F {
		return 0x8000
	}
	return c << 8
}

func pairCode(prev, next int) int {
	return prev<<16 | next
}

// tokenBefore emits a space when the previous and next tokens would be
// parsed differently without it, and returns the code of the next token.
func (g *generator) tokenBefore(prevCode int, t TokenType, value string) int {
	nextCode := tokenCode(t, value)
	nextChar := firstUnit(value)

	var emitWs bool
	if (nextChar == '-' && t != IdentToken && t != FunctionToken && t != CDCToken) || nextChar == '+' {
		emitWs = whiteSpaceRequired[pairCode(prevCode, charCode(nextChar))]
	} else {
		emitWs = whiteSpaceRequired[pairCode(prevCode, nextCode)]
	}

	if emitWs {
		g.emit(" ", WhiteSpaceToken, true)
	}
	return nextCode
}

func buildWhiteSpacePairs() map[int]bool {
	d := func(c byte) int { return charCode(int(c)) }
	t := func(tt TokenType) int { return int(tt) }

	// https://www.w3.org/TR/css-syntax-3/#serialization
	var pairs [][2]int
	add := func(prev int, nexts ...int) {
		for _, next := range nexts {
			pairs = append(pairs, [2]int{prev, next})
		}
	}
	common := []int{t(IdentToken), t(FunctionToken), t(URLToken), t(BadURLToken), d('-'),
		t(NumberToken), t(PercentageToken), t(DimensionToken), t(CDCToken)}

	add(t(IdentToken), append(common, t(LeftParenthesisToken))...)
	add(t(AtKeywordToken), common...)
	add(t(HashToken), common...)
	add(t(DimensionToken), common...)
	add(d('#'), common...)
	add(d('-'), common...)
	add(t(NumberToken), t(IdentToken), t(FunctionToken), t(URLToken), t(BadURLToken),
		t(NumberToken), t(PercentageToken), t(DimensionToken), d('%'), t(CDCToken))
	add(d('@'), t(IdentToken), t(FunctionToken), t(URLToken), t(BadURLToken), d('-'), t(CDCToken))
	add(d('.'), t(NumberToken), t(PercentageToken), t(DimensionToken))
	add(d('+'), t(NumberToken), t(PercentageToken), t(DimensionToken))
	add(d('/'), d('*'))

	// safe mode additions
	add(t(IdentToken), t(HashToken))
	add(t(DimensionToken), t(HashToken))
	add(t(HashToken), t(HashToken))
	add(t(AtKeywordToken), t(LeftParenthesisToken), t(StringToken), t(ColonToken))
	add(t(PercentageToken), t(PercentageToken), t(DimensionToken), t(FunctionToken), d('-'))
	add(t(RightParenthesisToken), t(IdentToken), t(FunctionToken), t(PercentageToken),
		t(DimensionToken), t(HashToken), d('-'))

	set := make(map[int]bool, len(pairs))
	for _, pair := range pairs {
		set[pairCode(pair[0], pair[1])] = true
	}
	return set
}
