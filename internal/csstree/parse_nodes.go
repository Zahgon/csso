package csstree

import "strconv"

// This file ports the parse() functions of css-tree's lib/syntax/node/*.js.

func (p *parser) base(start, end int) Base {
	return Base{Location: p.getLocation(start, end)}
}

// parseRaw consumes tokens from startToken until consumeUntil stops (or
// the enclosing block ends) and returns them as a Raw node, optionally
// without trailing whitespace.
func (p *parser) parseRaw(startToken int, consumeUntil func(code int) int, excludeWhiteSpace bool) *Raw {
	startOffset := p.getTokenStart(startToken)
	if consumeUntil == nil {
		consumeUntil = consumeUntilBalanceEnd
	}
	p.skipUntilBalanced(startToken, consumeUntil)

	endOffset := p.tokenStart
	if excludeWhiteSpace && p.tokenStart > startOffset {
		endOffset = p.offsetExcludeWS()
	}
	return &Raw{Base: p.base(startOffset, endOffset), Value: p.substring(startOffset, endOffset)}
}

func (p *parser) offsetExcludeWS() int {
	if p.tokenIndex > 0 && p.lookupType(-1) == WhiteSpaceToken {
		if p.tokenIndex > 1 {
			return p.getTokenStart(p.tokenIndex - 1)
		}
		return p.firstCharOffset
	}
	return p.tokenStart
}

func (p *parser) parseStyleSheet() *StyleSheet {
	start := p.tokenStart
	children := NewList()
	rawFallback := func(startToken int) Node { return p.parseRaw(startToken, nil, false) }

	for !p.eof {
		var child Node
		switch p.tokenType {
		case WhiteSpaceToken:
			p.next()
			continue
		case CommentToken:
			// only exclamation comments (/*! ... */) are kept at the top level
			if p.charCodeAt(p.tokenStart+2) != '!' {
				p.next()
				continue
			}
			child = p.parseComment()
		case CDOToken:
			child = p.parseCDO()
		case CDCToken:
			child = p.parseCDC()
		case AtKeywordToken:
			child = p.parseWithFallback(func() Node { return p.parseAtrule() }, rawFallback)
		default:
			child = p.parseWithFallback(func() Node { return p.parseRule() }, rawFallback)
		}
		children.Push(child)
	}

	return &StyleSheet{Base: p.base(start, p.tokenStart), Children: children}
}

func (p *parser) parseAtrule() *Atrule {
	start := p.tokenStart
	var prelude Node
	var block *Block
	rawFallback := func(startToken int) Node {
		return p.parseRaw(startToken, consumeUntilLeftCurlyBracketOrSemicolon, true)
	}

	p.eat(AtKeywordToken)
	name := p.substrToCursor(start + 1)
	nameLowerCase := LowerCase(name)
	p.skipSC()

	if !p.eof && p.tokenType != LeftCurlyBracketToken && p.tokenType != SemicolonToken {
		if p.flags.atrulePrelude {
			prelude = p.parseWithFallback(func() Node { return p.parseAtrulePrelude(name) }, rawFallback)
		} else {
			prelude = rawFallback(p.tokenIndex)
		}
		p.skipSC()
	}

	switch p.tokenType {
	case SemicolonToken:
		p.next()
	case LeftCurlyBracketToken:
		isDeclaration, known := atruleBlockKind(nameLowerCase)
		if !known {
			isDeclaration = p.isDeclarationBlockAtrule()
		}
		block = p.parseBlock(isDeclaration)
	}

	return &Atrule{Base: p.base(start, p.tokenStart), Name: name, Prelude: prelude, Block: block}
}

// isDeclarationBlockAtrule guesses whether the block of an unknown at-rule
// holds declarations: a `}` met before any `{` or at-keyword says so.
func (p *parser) isDeclarationBlockAtrule() bool {
	for offset := 1; ; offset++ {
		switch p.lookupType(offset) {
		case EOFToken:
			return false
		case RightCurlyBracketToken:
			return true
		case LeftCurlyBracketToken, AtKeywordToken:
			return false
		}
	}
}

func (p *parser) parseAtrulePrelude(name string) *AtrulePrelude {
	p.skipSC()

	var children *List
	if custom := atrulePreludeParser(LowerCase(name)); custom != nil {
		children = custom(p)
	} else {
		children = p.readSequence(atrulePreludeScope)
	}

	p.skipSC()
	if !p.eof && p.tokenType != LeftCurlyBracketToken && p.tokenType != SemicolonToken {
		p.fail("Semicolon or block is expected")
	}

	return &AtrulePrelude{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseRule() *Rule {
	startToken := p.tokenIndex
	startOffset := p.tokenStart
	rawFallback := func(startToken int) Node {
		return p.parseRaw(startToken, consumeUntilLeftCurlyBracket, true)
	}

	var prelude Node
	if p.flags.rulePrelude {
		prelude = p.parseWithFallback(func() Node {
			selectors := p.parseSelectorList()
			if !p.eof && p.tokenType != LeftCurlyBracketToken {
				p.fail("")
			}
			return selectors
		}, rawFallback)
	} else {
		prelude = rawFallback(startToken)
	}

	block := p.parseBlock(true)
	return &Rule{Base: p.base(startOffset, p.tokenStart), Prelude: prelude, Block: block}
}

func (p *parser) parseBlock(isDeclaration bool) *Block {
	start := p.tokenStart
	children := NewList()
	rawFallback := func(startToken int) Node { return p.parseRaw(startToken, nil, true) }
	rawDeclaration := func(startToken int) Node {
		return p.parseRaw(startToken, consumeUntilSemicolonIncluded, true)
	}

	consume := func() Node {
		return p.parseWithFallback(func() Node { return p.parseRule() }, rawFallback)
	}
	if isDeclaration {
		consume = func() Node {
			if p.tokenType == SemicolonToken {
				return rawDeclaration(p.tokenIndex)
			}
			node := p.parseWithFallback(func() Node { return p.parseDeclaration() }, rawDeclaration)
			if p.tokenType == SemicolonToken {
				p.next()
			}
			return node
		}
	}

	p.eat(LeftCurlyBracketToken)

scan:
	for !p.eof {
		switch p.tokenType {
		case RightCurlyBracketToken:
			break scan
		case WhiteSpaceToken, CommentToken:
			p.next()
		case AtKeywordToken:
			children.Push(p.parseWithFallback(func() Node { return p.parseAtrule() }, rawFallback))
		default:
			children.Push(consume())
		}
	}

	if !p.eof {
		p.eat(RightCurlyBracketToken)
	}

	return &Block{Base: p.base(start, p.tokenStart), Children: children}
}

func (p *parser) parseDeclarationList() *DeclarationList {
	children := NewList()
	rawFallback := func(startToken int) Node {
		return p.parseRaw(startToken, consumeUntilSemicolonIncluded, true)
	}

	for !p.eof {
		switch p.tokenType {
		case WhiteSpaceToken, CommentToken, SemicolonToken:
			p.next()
		default:
			children.Push(p.parseWithFallback(func() Node { return p.parseDeclaration() }, rawFallback))
		}
	}

	return &DeclarationList{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseDeclaration() *Declaration {
	start := p.tokenStart
	startToken := p.tokenIndex
	property := p.readProperty()
	customProperty := IsCustomProperty(property)
	parseValue := p.flags.value
	excludeWhiteSpace := true
	if customProperty {
		parseValue = p.flags.customProperty
		excludeWhiteSpace = false
	}
	rawFallback := func(startToken int) Node {
		return p.parseRaw(startToken, consumeUntilExclamationMarkOrSemicolon, excludeWhiteSpace)
	}

	p.skipSC()
	p.eat(ColonToken)

	valueStart := p.tokenIndex
	if !customProperty {
		p.skipSC()
	}

	var value Node
	if parseValue {
		value = p.parseWithFallback(func() Node {
			startValueToken := p.tokenIndex
			value := p.parseValue()
			if !p.eof &&
				p.tokenType != SemicolonToken &&
				!p.isDelim('!') &&
				!p.isBalanceEdge(startValueToken) {
				p.fail("")
			}
			return value
		}, rawFallback)
	} else {
		value = rawFallback(p.tokenIndex)
	}

	if v, ok := value.(*Value); ok && customProperty && v.Children.IsEmpty() {
		p.appendSkippedWhiteSpace(v, valueStart)
	}

	important := ""
	if p.isDelim('!') {
		// css-tree stores `true` for exactly "important" and the identifier
		// otherwise; both cases are the identifier text.
		p.eat(DelimToken)
		p.skipSC()
		important = p.consume(IdentToken)
		p.skipSC()
	}

	// the semicolon is not part of the declaration range
	if !p.eof && p.tokenType != SemicolonToken && !p.isBalanceEdge(startToken) {
		p.fail("")
	}

	return &Declaration{
		Base:      p.base(start, p.tokenStart),
		Important: important,
		Property:  property,
		Value:     value,
	}
}

// appendSkippedWhiteSpace keeps an otherwise empty custom property value
// (or var() fallback) meaningful: when whitespace was skipped since
// startToken, a single WhiteSpace node is added.
func (p *parser) appendSkippedWhiteSpace(v *Value, startToken int) {
	for offset := startToken - p.tokenIndex; offset <= 0; offset++ {
		if p.lookupType(offset) == WhiteSpaceToken {
			v.Children.AppendData(&WhiteSpace{Value: " "})
			return
		}
	}
}

// readProperty reads a property name including the IE hacks css-tree
// supports (`*`, `$`, `+`, `#`, `&`, `/` and `//` prefixes).
func (p *parser) readProperty() string {
	start := p.tokenStart

	if p.tokenType == DelimToken {
		switch p.charCodeAt(p.tokenStart) {
		case '*', '$', '+', '#', '&':
			p.next()
		case '/':
			p.next()
			if p.isDelim('/') {
				p.next()
			}
		}
	}

	if p.tokenType == HashToken {
		p.eat(HashToken)
	} else {
		p.eat(IdentToken)
	}

	return p.substrToCursor(start)
}

func (p *parser) parseValue() *Value {
	start := p.tokenStart
	children := p.readSequence(valueScope)
	return &Value{Base: p.base(start, p.tokenStart), Children: children}
}

func (p *parser) parseSelectorList() *SelectorList {
	children := NewList()
	for !p.eof {
		children.Push(p.parseSelector())
		if p.tokenType == CommaToken {
			p.next()
			continue
		}
		break
	}
	return &SelectorList{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseSelector() *Selector {
	children := p.readSequence(selectorScope)
	if children.First() == nil {
		p.fail("Selector is expected")
	}
	return &Selector{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseIdentifier() *Identifier {
	loc := p.getLocation(p.tokenStart, p.tokenEnd)
	return &Identifier{Base: Base{Location: loc}, Name: p.consume(IdentToken)}
}

func (p *parser) parseNumber() *Number {
	loc := p.getLocation(p.tokenStart, p.tokenEnd)
	return &Number{Base: Base{Location: loc}, Value: p.consume(NumberToken)}
}

func (p *parser) parseDimension() *Dimension {
	start := p.tokenStart
	value := p.consumeNumber(DimensionToken)
	return &Dimension{
		Base:  p.base(start, p.tokenStart),
		Value: value,
		Unit:  p.substring(start+UTF16Len(value), p.tokenStart),
	}
}

func (p *parser) parsePercentage() *Percentage {
	loc := p.getLocation(p.tokenStart, p.tokenEnd)
	return &Percentage{Base: Base{Location: loc}, Value: p.consumeNumber(PercentageToken)}
}

func (p *parser) parseString() *String {
	loc := p.getLocation(p.tokenStart, p.tokenEnd)
	return &String{Base: Base{Location: loc}, Value: DecodeString(p.consume(StringToken))}
}

// parseURL parses <url-token> or url( <string> ).
func (p *parser) parseURL() *Url {
	start := p.tokenStart
	var value string

	switch p.tokenType {
	case URLToken:
		value = DecodeURL(p.consume(URLToken))
	case FunctionToken:
		if !p.cmpStr(p.tokenStart, p.tokenEnd, "url(") {
			p.fail("Function name must be `url`")
		}
		p.eat(FunctionToken)
		p.skipSC()
		value = DecodeString(p.consume(StringToken))
		p.skipSC()
		if !p.eof {
			p.eat(RightParenthesisToken)
		}
	default:
		p.fail("Url or Function is expected")
	}

	return &Url{Base: p.base(start, p.tokenStart), Value: value}
}

func (p *parser) parseHash() *Hash {
	start := p.tokenStart
	p.eat(HashToken)
	return &Hash{Base: p.base(start, p.tokenStart), Value: p.substrToCursor(start + 1)}
}

func (p *parser) parseFunction(readSequence sequenceReader, rc recognizer) *Function {
	start := p.tokenStart
	name := p.consumeFunctionName()

	var children *List
	if custom := rc.functionReader(LowerCase(name)); custom != nil {
		children = custom(p, rc)
	} else {
		children = readSequence(p, rc)
	}

	if !p.eof {
		p.eat(RightParenthesisToken)
	}

	return &Function{Base: p.base(start, p.tokenStart), Name: name, Children: children}
}

func (p *parser) parseOperator() *Operator {
	start := p.tokenStart
	p.next()
	return &Operator{Base: p.base(start, p.tokenStart), Value: p.substrToCursor(start)}
}

func (p *parser) parseParentheses(readSequence sequenceReader, rc recognizer) *Parentheses {
	start := p.tokenStart
	p.eat(LeftParenthesisToken)
	children := readSequence(p, rc)
	if !p.eof {
		p.eat(RightParenthesisToken)
	}
	return &Parentheses{Base: p.base(start, p.tokenStart), Children: children}
}

func (p *parser) parseBrackets(readSequence sequenceReader, rc recognizer) *Brackets {
	start := p.tokenStart
	p.eat(LeftSquareBracketToken)
	children := readSequence(p, rc)
	if !p.eof {
		p.eat(RightSquareBracketToken)
	}
	return &Brackets{Base: p.base(start, p.tokenStart), Children: children}
}

func (p *parser) parseComment() *Comment {
	start := p.tokenStart
	end := p.tokenEnd
	p.eat(CommentToken)
	if end-start+2 >= 2 && p.charCodeAt(end-2) == '*' && p.charCodeAt(end-1) == '/' {
		end -= 2
	}
	return &Comment{Base: p.base(start, p.tokenStart), Value: p.substring(start+2, end)}
}

func (p *parser) parseCDO() *CDO {
	start := p.tokenStart
	p.eat(CDOToken)
	return &CDO{Base: p.base(start, p.tokenStart)}
}

func (p *parser) parseCDC() *CDC {
	start := p.tokenStart
	p.eat(CDCToken)
	return &CDC{Base: p.base(start, p.tokenStart)}
}

// parseWhiteSpace consumes a whitespace token. Like css-tree it yields a
// location-less node holding a single space.
func (p *parser) parseWhiteSpace() *WhiteSpace {
	p.eat(WhiteSpaceToken)
	return &WhiteSpace{Value: " "}
}

// parseTypeSelector parses `ident`, `*`, and their namespaced forms
// (`ns|ident`, `*|*`, `|ident`, ...).
func (p *parser) parseTypeSelector() *TypeSelector {
	start := p.tokenStart
	eatIdentifierOrAsterisk := func() {
		if p.tokenType != IdentToken && !p.isDelim('*') {
			p.fail("Identifier or asterisk is expected")
		}
		p.next()
	}

	if p.isDelim('|') {
		p.next()
		eatIdentifierOrAsterisk()
	} else {
		eatIdentifierOrAsterisk()
		if p.isDelim('|') {
			p.next()
			eatIdentifierOrAsterisk()
		}
	}

	return &TypeSelector{Base: p.base(start, p.tokenStart), Name: p.substrToCursor(start)}
}

func (p *parser) parseIDSelector() *IdSelector {
	start := p.tokenStart
	p.eat(HashToken)
	return &IdSelector{Base: p.base(start, p.tokenStart), Name: p.substrToCursor(start + 1)}
}

func (p *parser) parseClassSelector() *ClassSelector {
	p.eatDelim('.')
	loc := p.getLocation(p.tokenStart-1, p.tokenEnd)
	return &ClassSelector{Base: Base{Location: loc}, Name: p.consume(IdentToken)}
}

func (p *parser) parseAttributeSelector() *AttributeSelector {
	start := p.tokenStart
	node := &AttributeSelector{}

	p.eat(LeftSquareBracketToken)
	p.skipSC()
	node.Name = p.attributeName()
	p.skipSC()

	if p.tokenType != RightSquareBracketToken {
		// a bare identifier here is a flag, as in `[name i]`
		if p.tokenType != IdentToken {
			node.Matcher = p.attributeMatcher()
			p.skipSC()
			if p.tokenType == StringToken {
				node.Value = p.parseString()
			} else {
				node.Value = p.parseIdentifier()
			}
			p.skipSC()
		}
		if p.tokenType == IdentToken {
			node.Flags = p.consume(IdentToken)
			p.skipSC()
		}
	}

	p.eat(RightSquareBracketToken)
	node.Location = p.getLocation(start, p.tokenStart)
	return node
}

// attributeName parses a <wq-name> (optionally namespaced attribute name).
func (p *parser) attributeName() *Identifier {
	if p.eof {
		p.fail("Unexpected end of input")
	}

	start := p.tokenStart
	expectIdent := false

	if p.isDelim('*') {
		expectIdent = true
		p.next()
	} else if !p.isDelim('|') {
		p.eat(IdentToken)
	}

	if p.isDelim('|') {
		if p.charCodeAt(p.tokenStart+1) != '=' {
			p.next()
			p.eat(IdentToken)
		} else if expectIdent {
			p.failAt("Identifier is expected", p.tokenEnd)
		}
	} else if expectIdent {
		p.fail("Vertical line is expected")
	}

	return &Identifier{Base: p.base(start, p.tokenStart), Name: p.substrToCursor(start)}
}

// attributeMatcher parses one of =, ~=, ^=, $=, *=, |=.
func (p *parser) attributeMatcher() string {
	start := p.tokenStart
	code := p.charCodeAt(start)

	switch code {
	case '=', '~', '^', '$', '*', '|':
	default:
		p.fail("Attribute selector (=, ~=, ^=, $=, *=, |=) is expected")
	}

	p.next()
	if code != '=' {
		if !p.isDelim('=') {
			p.fail("Equal sign is expected")
		}
		p.next()
	}

	return p.substrToCursor(start)
}

func (p *parser) parsePseudoClassSelector() *PseudoClassSelector {
	start := p.tokenStart
	p.eat(ColonToken)
	name, children := p.pseudoNameAndChildren()
	return &PseudoClassSelector{Base: p.base(start, p.tokenStart), Name: name, Children: children}
}

func (p *parser) parsePseudoElementSelector() *PseudoElementSelector {
	start := p.tokenStart
	p.eat(ColonToken)
	p.eat(ColonToken)
	name, children := p.pseudoNameAndChildren()
	return &PseudoElementSelector{Base: p.base(start, p.tokenStart), Name: name, Children: children}
}

// pseudoNameAndChildren parses `<ident>` or `<function-token> ... )` after
// the colon(s) of a pseudo selector. Known functional pseudos get their
// argument parsed; others keep it as a Raw node.
func (p *parser) pseudoNameAndChildren() (string, *List) {
	if p.tokenType != FunctionToken {
		return p.consume(IdentToken), nil
	}

	name := p.consumeFunctionName()
	var children *List
	if custom := pseudoParser(LowerCase(name)); custom != nil {
		p.skipSC()
		children = custom(p)
		p.skipSC()
	} else {
		children = NewListOf(p.parseRaw(p.tokenIndex, nil, false))
	}
	p.eat(RightParenthesisToken)
	return name, children
}

// parseCombinator parses `+`, `>`, `~` or `/deep/`.
func (p *parser) parseCombinator() *Combinator {
	start := p.tokenStart
	var name string

	switch p.tokenType {
	case WhiteSpaceToken:
		name = " "
	case DelimToken:
		switch p.charCodeAt(p.tokenStart) {
		case '>', '+', '~':
			p.next()
		case '/':
			p.next()
			p.eatIdent("deep")
			p.eatDelim('/')
		default:
			p.fail("Combinator is expected")
		}
		name = p.substrToCursor(start)
	}

	return &Combinator{Base: p.base(start, p.tokenStart), Name: name}
}

func (p *parser) parseNth() *Nth {
	p.skipSC()

	start := p.tokenStart
	var nth Node
	var selector *SelectorList

	if p.lookupValue(0, "odd") || p.lookupValue(0, "even") {
		nth = p.parseIdentifier()
	} else {
		nth = p.parseAnPlusB()
	}

	end := p.tokenStart
	p.skipSC()

	if p.lookupValue(0, "of") {
		p.next()
		selector = p.parseSelectorList()
		end = p.tokenStart
	}

	return &Nth{Base: p.base(start, end), Nth: nth, Selector: selector}
}

func (p *parser) parseMediaQueryList() *MediaQueryList {
	children := NewList()
	p.skipSC()
	for !p.eof {
		children.Push(p.parseMediaQuery())
		if p.tokenType != CommaToken {
			break
		}
		p.next()
	}
	return &MediaQueryList{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseMediaQuery() *MediaQuery {
	children := NewList()
	var child Node

	p.skipSC()

scan:
	for !p.eof {
		switch p.tokenType {
		case CommentToken, WhiteSpaceToken:
			p.next()
			continue
		case IdentToken:
			child = p.parseIdentifier()
		case LeftParenthesisToken:
			child = p.parseMediaFeature()
		default:
			break scan
		}
		children.Push(child)
	}

	if child == nil {
		p.fail("Identifier or parenthesis is expected")
	}

	return &MediaQuery{Base: Base{Location: p.getLocationFromList(children)}, Children: children}
}

func (p *parser) parseMediaFeature() *MediaFeature {
	start := p.tokenStart
	var value Node

	p.eat(LeftParenthesisToken)
	p.skipSC()

	name := p.consume(IdentToken)
	p.skipSC()

	if p.tokenType != RightParenthesisToken {
		p.eat(ColonToken)
		p.skipSC()

		switch p.tokenType {
		case NumberToken:
			if p.lookupNonWSType(1) == DelimToken {
				value = p.parseRatio()
			} else {
				value = p.parseNumber()
			}
		case DimensionToken:
			value = p.parseDimension()
		case IdentToken:
			value = p.parseIdentifier()
		default:
			p.fail("Number, dimension, ratio or identifier is expected")
		}

		p.skipSC()
	}

	p.eat(RightParenthesisToken)
	return &MediaFeature{Base: p.base(start, p.tokenStart), Name: name, Value: value}
}

// parseRatio parses `<number> / <number>`. Terms are only checked to be
// unsigned, non-zero numbers without an exponent (css-tree relaxes the spec
// for -o-min-device-pixel-ratio).
func (p *parser) parseRatio() *Ratio {
	start := p.tokenStart
	left := p.ratioTerm()
	p.skipSC()
	p.eatDelim('/')
	right := p.ratioTerm()
	return &Ratio{Base: p.base(start, p.tokenStart), Left: left, Right: right}
}

func (p *parser) ratioTerm() string {
	p.skipSC()
	value := p.consume(NumberToken)
	for i := 0; i < len(value); i++ {
		if !IsDigit(int(value[i])) && value[i] != '.' {
			p.failAt("Unsigned number is expected", p.tokenStart-len(value)+i)
		}
	}
	if n, err := strconv.ParseFloat(value, 64); err == nil && n == 0 {
		p.failAt("Zero number is not allowed", p.tokenStart-len(value))
	}
	return value
}
