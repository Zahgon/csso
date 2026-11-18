package csstree

// This file ports css-tree's parsing scopes (lib/syntax/scope), function
// handlers (lib/syntax/function), at-rule handlers (lib/syntax/atrule) and
// pseudo selector handlers (lib/syntax/pseudo).

// sequenceReader reads the children of a Function, Parentheses or Brackets
// node.
type sequenceReader func(p *parser, rc recognizer) *List

// recognizer is a css-tree parsing scope: it picks the node parser for the
// current token and decides what whitespace between nodes means.
type recognizer int

const (
	atrulePreludeScope recognizer = iota
	selectorScope
	valueScope
)

// getNode parses the node at the current token, or returns nil when the
// token ends the sequence.
func (rc recognizer) getNode(p *parser) Node {
	if rc == selectorScope {
		return selectorGetNode(p)
	}
	return defaultGetNode(p, rc)
}

// onWhiteSpace is called when whitespace preceded next (nil at the end of
// the sequence); the at-rule prelude scope ignores whitespace.
func (rc recognizer) onWhiteSpace(next Node, children *List) {
	switch rc {
	case selectorScope:
		selectorOnWhiteSpace(next, children)
	case valueScope:
		valueOnWhiteSpace(next, children)
	}
}

// functionReader returns the scope's dedicated reader for the arguments of
// the function with the given lowercase name, or nil.
func (rc recognizer) functionReader(name string) sequenceReader {
	if rc != valueScope {
		return nil
	}
	switch name {
	case "expression":
		return readExpression
	case "var":
		return readVar
	}
	return nil
}

// readSequence parses nodes with rc until it recognizes no more; comments
// are skipped and whitespace is reported to rc.onWhiteSpace.
func (p *parser) readSequence(rc recognizer) *List {
	children := NewList()
	space := false

	for !p.eof {
		switch p.tokenType {
		case CommentToken:
			p.next()
			continue
		case WhiteSpaceToken:
			space = true
			p.next()
			continue
		}

		child := rc.getNode(p)
		if child == nil {
			break
		}

		if space {
			rc.onWhiteSpace(child, children)
			space = false
		}

		children.Push(child)
	}

	if space {
		rc.onWhiteSpace(nil, children)
	}

	return children
}

// defaultGetNode is the recognizer of values and at-rule preludes.
func defaultGetNode(p *parser, rc recognizer) Node {
	switch p.tokenType {
	case HashToken:
		return p.parseHash()
	case CommaToken:
		return p.parseOperator()
	case LeftParenthesisToken:
		return p.parseParentheses((*parser).readSequence, rc)
	case LeftSquareBracketToken:
		return p.parseBrackets((*parser).readSequence, rc)
	case StringToken:
		return p.parseString()
	case DimensionToken:
		return p.parseDimension()
	case PercentageToken:
		return p.parsePercentage()
	case NumberToken:
		return p.parseNumber()
	case FunctionToken:
		if p.cmpStr(p.tokenStart, p.tokenEnd, "url(") {
			return p.parseURL()
		}
		return p.parseFunction((*parser).readSequence, rc)
	case URLToken:
		return p.parseURL()
	case IdentToken:
		// a unicode range starts with u+ or U+
		if p.cmpChar(p.tokenStart, 'u') && p.cmpChar(p.tokenStart+1, '+') {
			return p.parseUnicodeRange()
		}
		return p.parseIdentifier()
	case DelimToken:
		switch p.charCodeAt(p.tokenStart) {
		case '/', '*', '+', '-':
			return p.parseOperator()
		case '#':
			p.failAt("Hex or identifier is expected", p.tokenStart+1)
		}
	}
	return nil
}

func isPlusMinusOperator(node Node) (*Operator, bool) {
	op, ok := node.(*Operator)
	if !ok || op.Value == "" {
		return nil, false
	}
	last := op.Value[len(op.Value)-1]
	return op, last == '-' || last == '+'
}

// valueOnWhiteSpace keeps the whitespace around `+` and `-` operators, which
// is significant in calc().
func valueOnWhiteSpace(next Node, children *List) {
	if op, ok := isPlusMinusOperator(next); ok {
		op.Value = " " + op.Value
	}
	if op, ok := isPlusMinusOperator(children.Last()); ok {
		op.Value += " "
	}
}

// selectorGetNode is the recognizer of selectors.
func selectorGetNode(p *parser) Node {
	switch p.tokenType {
	case LeftSquareBracketToken:
		return p.parseAttributeSelector()
	case HashToken:
		return p.parseIDSelector()
	case ColonToken:
		if p.lookupType(1) == ColonToken {
			return p.parsePseudoElementSelector()
		}
		return p.parsePseudoClassSelector()
	case IdentToken:
		return p.parseTypeSelector()
	case NumberToken, PercentageToken:
		return p.parsePercentage()
	case DimensionToken:
		// .123ident is not a class selector
		if p.charCodeAt(p.tokenStart) == '.' {
			p.failAt("Identifier is expected", p.tokenStart+1)
		}
	case DelimToken:
		switch p.charCodeAt(p.tokenStart) {
		case '+', '>', '~', '/':
			return p.parseCombinator()
		case '.':
			return p.parseClassSelector()
		case '*', '|':
			return p.parseTypeSelector()
		case '#':
			return p.parseIDSelector()
		}
	}
	return nil
}

// selectorOnWhiteSpace turns whitespace between two compound selectors into
// a descendant combinator (a location-less node, as in css-tree).
func selectorOnWhiteSpace(next Node, children *List) {
	last := children.Last()
	if last != nil && last.Type() != "Combinator" && next != nil && next.Type() != "Combinator" {
		children.Push(&Combinator{Name: " "})
	}
}

// readExpression reads the argument of the legacy IE expression() function
// as a single Raw node.
func readExpression(p *parser, _ recognizer) *List {
	return NewListOf(p.parseRaw(p.tokenIndex, nil, false))
}

// readVar reads `var( <ident> , <value>? )`; only the first argument is
// checked to be an identifier.
func readVar(p *parser, _ recognizer) *List {
	children := NewList()

	p.skipSC()
	children.Push(p.parseIdentifier())
	p.skipSC()

	if p.tokenType == CommaToken {
		children.Push(p.parseOperator())

		startIndex := p.tokenIndex
		var value Node
		if p.flags.customProperty {
			v := p.parseValue()
			if v.Children.IsEmpty() {
				p.appendSkippedWhiteSpace(v, startIndex)
			}
			value = v
		} else {
			value = p.parseRaw(p.tokenIndex, consumeUntilExclamationMarkOrSemicolon, false)
		}
		children.Push(value)
	}

	return children
}

// atrulePreludeParser returns the dedicated prelude grammar of a known
// at-rule (by lowercase name), or nil.
func atrulePreludeParser(name string) func(p *parser) *List {
	switch name {
	case "import":
		return importPrelude
	case "media":
		return func(p *parser) *List { return NewListOf(p.parseMediaQueryList()) }
	case "page":
		return func(p *parser) *List { return NewListOf(p.parseSelectorList()) }
	case "supports":
		return supportsPrelude
	}
	return nil
}

// atruleBlockKind reports whether a known at-rule (by lowercase name) has a
// declaration block; known is false for at-rules whose block kind css-tree
// guesses with isDeclarationBlockAtrule.
func atruleBlockKind(name string) (isDeclaration, known bool) {
	switch name {
	case "font-face", "page":
		return true, true
	case "media", "supports":
		return false, true
	}
	return false, false
}

// importPrelude parses `<string> | <url>` followed by an optional media
// query list.
func importPrelude(p *parser) *List {
	children := NewList()

	p.skipSC()

	switch p.tokenType {
	case StringToken:
		children.Push(p.parseString())
	case URLToken, FunctionToken:
		children.Push(p.parseURL())
	default:
		p.fail("String or url() is expected")
	}

	if p.lookupNonWSType(0) == IdentToken || p.lookupNonWSType(0) == LeftParenthesisToken {
		children.Push(p.parseMediaQueryList())
	}

	return children
}

func supportsPrelude(p *parser) *List {
	children := supportsSequence(p, atrulePreludeScope)
	if children.First() == nil {
		p.fail("Condition is expected")
	}
	return children
}

// supportsSequence reads the identifiers, functions and parenthesized
// conditions of an @supports prelude.
func supportsSequence(p *parser, _ recognizer) *List {
	children := NewList()

	p.skipSC()

scan:
	for !p.eof {
		var child Node
		switch p.tokenType {
		case CommentToken, WhiteSpaceToken:
			p.next()
			continue
		case FunctionToken:
			child = p.parseFunction(supportsRawArgument, atrulePreludeScope)
		case IdentToken:
			child = p.parseIdentifier()
		case LeftParenthesisToken:
			child = p.parseParentheses(supportsParentheses, atrulePreludeScope)
		default:
			break scan
		}
		children.Push(child)
	}

	return children
}

func supportsRawArgument(p *parser, _ recognizer) *List {
	return NewListOf(p.parseRaw(p.tokenIndex, nil, false))
}

// supportsParentheses parses a declaration test such as `(display: grid)`
// or a nested condition.
func supportsParentheses(p *parser, rc recognizer) *List {
	p.skipSC()
	if p.tokenType == IdentToken && p.lookupNonWSType(1) == ColonToken {
		return NewListOf(p.parseDeclaration())
	}
	return supportsSequence(p, rc)
}

// pseudoParser returns the argument grammar of a known functional pseudo
// class or element (by lowercase name), or nil.
func pseudoParser(name string) func(p *parser) *List {
	switch name {
	case "dir", "lang":
		return identListPseudo
	case "has", "matches", "is", "-moz-any", "-webkit-any", "where", "not":
		return selectorListPseudo
	case "nth-child", "nth-last-child", "nth-last-of-type", "nth-of-type":
		return nthPseudo
	case "slotted":
		return func(p *parser) *List { return NewListOf(p.parseSelector()) }
	}
	return nil
}

func selectorListPseudo(p *parser) *List {
	return NewListOf(p.parseSelectorList())
}

func identListPseudo(p *parser) *List {
	return NewListOf(p.parseIdentifier())
}

func nthPseudo(p *parser) *List {
	return NewListOf(p.parseNth())
}
