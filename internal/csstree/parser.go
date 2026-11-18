package csstree

import (
	"fmt"
	"strings"
)

// Parse contexts accepted by ParseOptions.Context (css-tree's parseContext).
const (
	ContextDefault         = "default" // same as ContextStylesheet
	ContextStylesheet      = "stylesheet"
	ContextAtrule          = "atrule"
	ContextAtrulePrelude   = "atrulePrelude"
	ContextMediaQueryList  = "mediaQueryList"
	ContextMediaQuery      = "mediaQuery"
	ContextRule            = "rule"
	ContextSelectorList    = "selectorList"
	ContextSelector        = "selector"
	ContextBlock           = "block"
	ContextDeclarationList = "declarationList"
	ContextDeclaration     = "declaration"
	ContextValue           = "value"
)

// ParseOptions configures Parse. The zero value parses a stylesheet the way
// css-tree's parse() does with no options.
type ParseOptions struct {
	// Context selects the grammar production to parse; "" means
	// ContextDefault.
	Context string
	// Atrule is the at-rule name used by ContextAtrulePrelude to pick a
	// prelude grammar (css-tree's `atrule` option).
	Atrule string

	// Positions makes the parser attach a Location to nodes.
	Positions bool
	// Filename is stored as Location.Source; "" means "<unknown>".
	Filename string
	// Offset, Line and Column give the position of the first character of
	// the source; zero Line and Column mean 1. With Line > 1, css-tree 2.2.0
	// fails with a TypeError instead of a syntax error when an error lies
	// past the source's own line count (it renders the error excerpt
	// eagerly); this port always reports the syntax error.
	Offset int
	Line   int
	Column int

	// RawAtrulePrelude keeps at-rule preludes as Raw nodes
	// (css-tree's parseAtrulePrelude: false).
	RawAtrulePrelude bool
	// RawRulePrelude keeps rule selectors as Raw nodes
	// (css-tree's parseRulePrelude: false).
	RawRulePrelude bool
	// RawValue keeps declaration values as Raw nodes
	// (css-tree's parseValue: false).
	RawValue bool
	// ParseCustomProperty parses custom property values and var() fallbacks
	// instead of keeping them as Raw nodes.
	ParseCustomProperty bool

	// OnParseError is called whenever the parser recovers from a syntax
	// error, with the error and the Raw node that replaced the broken part.
	OnParseError func(err *SyntaxError, fallback Node)
	// OnComment is called for every comment in the source before parsing,
	// with the comment text (without delimiters) and its location (nil
	// unless Positions is set).
	OnComment func(value string, loc *Location)
}

// parser holds the state of a single Parse call; it embeds the token stream
// like css-tree's parser object inherits from TokenStream.
type parser struct {
	tokenStream

	sourceText    string
	locationMap   offsetToLocation
	filename      string
	needPositions bool
	onParseError  func(err *SyntaxError, fallback Node)

	flags parseFlags
}

// Parse parses source according to opts (nil means default options) and
// returns the root node. Syntax errors in recoverable positions are turned
// into Raw nodes (reported through OnParseError); other syntax errors are
// returned as *SyntaxError.
func Parse(source string, opts *ParseOptions) (Node, error) {
	if opts == nil {
		opts = &ParseOptions{}
	}

	context := opts.Context
	if context == "" {
		context = ContextDefault
	}
	parseContext, ok := parseContexts[context]
	if !ok {
		return nil, fmt.Errorf("csstree: unknown context `%s`", context)
	}

	p := &parser{
		sourceText:    source,
		filename:      opts.Filename,
		needPositions: opts.Positions,
		onParseError:  opts.OnParseError,
		flags: parseFlags{
			atrulePrelude:  !opts.RawAtrulePrelude,
			rulePrelude:    !opts.RawRulePrelude,
			value:          !opts.RawValue,
			customProperty: opts.ParseCustomProperty,
		},
	}
	if p.filename == "" {
		p.filename = "<unknown>"
	}
	units := toUnits(source)
	p.setSource(units)
	line, column := opts.Line, opts.Column
	if line == 0 {
		line = 1
	}
	if column == 0 {
		column = 1
	}
	p.locationMap.setSource(units, opts.Offset, line, column)

	if opts.OnComment != nil {
		p.forEachToken(func(t TokenType, start, end int) {
			if t != CommentToken {
				return
			}
			loc := p.getLocation(start, end)
			valueEnd := end
			if cmpStr(units, end-2, end, "*/") {
				valueEnd = end - 2
			}
			// css-tree slices here, so a lone "/*/" yields an empty value
			value := ""
			if valueEnd > start+2 {
				value = p.substring(start+2, valueEnd)
			}
			opts.OnComment(value, loc)
		})
	}

	return catchSyntaxError(func() Node {
		ast := parseContext(p, opts)
		if !p.eof {
			p.fail("")
		}
		return ast
	})
}

// catchSyntaxError executes fn and converts a syntax error raised inside it
// into an error result.
func catchSyntaxError(fn func() Node) (node Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			syntaxErr, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			node, err = nil, syntaxErr
		}
	}()
	return fn(), nil
}

var parseContexts = map[string]func(p *parser, opts *ParseOptions) Node{
	ContextDefault:         func(p *parser, _ *ParseOptions) Node { return p.parseStyleSheet() },
	ContextStylesheet:      func(p *parser, _ *ParseOptions) Node { return p.parseStyleSheet() },
	ContextAtrule:          func(p *parser, _ *ParseOptions) Node { return p.parseAtrule() },
	ContextAtrulePrelude:   func(p *parser, opts *ParseOptions) Node { return p.parseAtrulePrelude(opts.Atrule) },
	ContextMediaQueryList:  func(p *parser, _ *ParseOptions) Node { return p.parseMediaQueryList() },
	ContextMediaQuery:      func(p *parser, _ *ParseOptions) Node { return p.parseMediaQuery() },
	ContextRule:            func(p *parser, _ *ParseOptions) Node { return p.parseRule() },
	ContextSelectorList:    func(p *parser, _ *ParseOptions) Node { return p.parseSelectorList() },
	ContextSelector:        func(p *parser, _ *ParseOptions) Node { return p.parseSelector() },
	ContextBlock:           func(p *parser, _ *ParseOptions) Node { return p.parseBlock(true) },
	ContextDeclarationList: func(p *parser, _ *ParseOptions) Node { return p.parseDeclarationList() },
	ContextDeclaration:     func(p *parser, _ *ParseOptions) Node { return p.parseDeclaration() },
	ContextValue:           func(p *parser, _ *ParseOptions) Node { return p.parseValue() },
}

// Stop conditions for skipUntilBalanced (css-tree's consumeUntil* helpers).

func consumeUntilBalanceEnd(int) int { return scanContinue }

func consumeUntilLeftCurlyBracket(code int) int {
	if code == '{' {
		return scanStop
	}
	return scanContinue
}

func consumeUntilLeftCurlyBracketOrSemicolon(code int) int {
	if code == '{' || code == ';' {
		return scanStop
	}
	return scanContinue
}

func consumeUntilExclamationMarkOrSemicolon(code int) int {
	if code == '!' || code == ';' {
		return scanStop
	}
	return scanContinue
}

func consumeUntilSemicolonIncluded(code int) int {
	if code == ';' {
		return scanStopIncluded
	}
	return scanContinue
}

// parseWithFallback runs consumer; when it raises a syntax error the token
// position is reset by fallback, which builds a replacement node starting at
// the token where consumer began.
func (p *parser) parseWithFallback(consumer func() Node, fallback func(startToken int) Node) (node Node) {
	startToken := p.tokenIndex
	defer func() {
		if r := recover(); r != nil {
			syntaxErr, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			node = fallback(startToken)
			if p.onParseError != nil {
				p.onParseError(syntaxErr, node)
			}
		}
	}()
	return consumer()
}

// lookupNonWSType returns the type of the first non-whitespace token at or
// after the relative offset.
func (p *parser) lookupNonWSType(offset int) TokenType {
	for {
		t := p.lookupType(offset)
		offset++
		if t != WhiteSpaceToken {
			return t
		}
	}
}

func (p *parser) charCodeAt(offset int) int {
	return unitAt(p.source, offset)
}

// substring follows String#substring: bounds are clamped and swapped when
// start > end (which css-tree relies on for a lone "/*/" comment).
func (p *parser) substring(start, end int) string {
	start = min(max(start, 0), len(p.source))
	end = min(max(end, 0), len(p.source))
	if start > end {
		start, end = end, start
	}
	return fromUnits(p.source[start:end])
}

func (p *parser) substrToCursor(start int) string {
	return p.substring(start, p.tokenStart)
}

func (p *parser) cmpChar(offset, code int) bool {
	return cmpChar(p.source, offset, code)
}

func (p *parser) cmpStr(start, end int, reference string) bool {
	return cmpStr(p.source, start, end, reference)
}

// consume eats a token of type t and returns its text.
func (p *parser) consume(t TokenType) string {
	start := p.tokenStart
	p.eat(t)
	return p.substrToCursor(start)
}

// consumeFunctionName eats a function token and returns its name (without
// the parenthesis).
func (p *parser) consumeFunctionName() string {
	name := p.substring(p.tokenStart, p.tokenEnd-1)
	p.eat(FunctionToken)
	return name
}

// consumeNumber eats a numeric token of type t and returns its numeric part.
func (p *parser) consumeNumber(t TokenType) string {
	number := p.substring(p.tokenStart, consumeNumber(p.source, p.tokenStart))
	p.eat(t)
	return number
}

// eat moves past the current token, which must be of type t.
func (p *parser) eat(t TokenType) {
	if p.tokenType == t {
		p.next()
		return
	}

	name := strings.ReplaceAll(strings.TrimSuffix(t.String(), "-token"), "-", " ")
	name = strings.ToUpper(name[:1]) + name[1:]
	message := name + " is expected"
	if strings.ContainsAny(name, "[](){}") {
		message = `"` + name + `" is expected`
	}
	offset := p.tokenStart

	switch t {
	case IdentToken:
		if p.tokenType == FunctionToken || p.tokenType == URLToken {
			offset = p.tokenEnd - 1
			message = "Identifier is expected but function found"
		} else {
			message = "Identifier is expected"
		}
	case HashToken:
		if p.isDelim('#') {
			p.next()
			offset++
			message = "Name is expected"
		}
	case PercentageToken:
		if p.tokenType == NumberToken {
			offset = p.tokenEnd
			message = "Percent sign is expected"
		}
	}

	p.failAt(message, offset)
}

// eatIdent moves past an identifier equal (ASCII case-insensitively) to name.
func (p *parser) eatIdent(name string) {
	if p.tokenType != IdentToken || !p.lookupValue(0, name) {
		p.fail(`Identifier "` + name + `" is expected`)
	}
	p.next()
}

// eatDelim moves past a delim token with the given code.
func (p *parser) eatDelim(code int) {
	if !p.isDelim(code) {
		p.fail(`Delim "` + string(rune(code)) + `" is expected`)
	}
	p.next()
}

func (p *parser) getLocation(start, end int) *Location {
	if !p.needPositions {
		return nil
	}
	return p.locationMap.locationRange(start, end, p.filename)
}

// getLocationFromList spans the locations of the first and last node of
// list, or is empty at the current token for an empty list.
func (p *parser) getLocationFromList(list *List) *Location {
	if !p.needPositions {
		return nil
	}
	start, end := p.tokenStart, p.tokenStart
	if head := list.First(); head != nil && head.Loc() != nil {
		start = head.Loc().Start.Offset - p.locationMap.startOffset
	}
	if tail := list.Last(); tail != nil && tail.Loc() != nil {
		end = tail.Loc().End.Offset - p.locationMap.startOffset
	}
	return p.locationMap.locationRange(start, end, p.filename)
}

// fail raises a syntax error at the current token ("Unexpected input" when
// message is empty). At the end of input the error points right after the
// last non-whitespace character.
func (p *parser) fail(message string) {
	var pos Position
	if p.eof {
		pos = p.locationMap.position(findWhiteSpaceStart(p.source, len(p.source)-1))
	} else {
		pos = p.locationMap.position(p.tokenStart)
	}
	p.raise(message, pos)
}

// failAt raises a syntax error at offset (falling back to fail's position
// when offset is past the end of the source).
func (p *parser) failAt(message string, offset int) {
	if offset >= len(p.source) {
		p.fail(message)
	}
	p.raise(message, p.locationMap.position(offset))
}

func (p *parser) raise(message string, pos Position) {
	if message == "" {
		message = "Unexpected input"
	}
	panic(&SyntaxError{
		Message: message,
		Source:  p.sourceText,
		Offset:  pos.Offset,
		Line:    pos.Line,
		Column:  pos.Column,
	})
}

// parseFlags select which parts of the input are parsed into nodes rather
// than kept as Raw.
type parseFlags struct {
	atrulePrelude  bool
	rulePrelude    bool
	value          bool
	customProperty bool
}
