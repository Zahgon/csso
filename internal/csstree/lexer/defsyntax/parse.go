package defsyntax

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/css/csso-go/internal/csstree"
)

const infinity = '∞'

// combinatorPrecedence orders combinators from the tightest to the loosest
// binding, as css-tree's COMBINATOR_PRECEDENCE does.
var combinatorPrecedence = map[string]int{
	" ":  1,
	"&&": 2,
	"||": 3,
	"|":  4,
}

// SyntaxError reports malformed definition syntax. Offset counts code points
// from the start of Input.
type SyntaxError struct {
	Message string
	Input   string
	Offset  int
}

// Error formats the error the way css-tree does: the message, the input and a
// caret under the offending position.
func (e *SyntaxError) Error() string {
	offset := e.Offset
	if offset == 0 {
		offset = utf8.RuneCountInString(e.Input)
	}
	return e.Message + "\n  " + e.Input + "\n--" + strings.Repeat("-", offset) + "^"
}

// combinatorTerm and spaces are transient parser tokens; regroupTerms removes
// every combinator and readImplicitGroup drops spaces, so neither escapes Parse.
type combinatorTerm struct{ value string }

type spaces struct{}

func (*combinatorTerm) Kind() string { return "Combinator" }
func (*spaces) Kind() string         { return "Spaces" }

// scanner walks the definition source by code point.
type scanner struct {
	src []rune
	pos int
}

func (s *scanner) charCodeAt(pos int) rune {
	if pos >= 0 && pos < len(s.src) {
		return s.src[pos]
	}
	return 0
}

func (s *scanner) charCode() rune     { return s.charCodeAt(s.pos) }
func (s *scanner) nextCharCode() rune { return s.charCodeAt(s.pos + 1) }

func (s *scanner) findWsEnd(pos int) int {
	for pos < len(s.src) && csstree.IsWhiteSpace(int(s.src[pos])) {
		pos++
	}
	return pos
}

func (s *scanner) substringToPos(end int) string {
	str := string(s.src[s.pos:end])
	s.pos = end
	return str
}

func (s *scanner) eat(code rune) {
	if s.charCode() != code {
		s.fail("Expect `" + string(code) + "`")
	}
	s.pos++
}

// peekChar consumes and returns the current character, or "" at the end.
func (s *scanner) peekChar() string {
	if s.pos < len(s.src) {
		s.pos++
		return string(s.src[s.pos-1])
	}
	return ""
}

// fail aborts parsing; Parse recovers the panic and returns the error.
func (s *scanner) fail(message string) {
	panic(&SyntaxError{Message: message, Input: string(s.src), Offset: s.pos})
}

// isNameChar reports membership in css-tree's NAME_CHAR table ([a-zA-Z0-9-]).
func isNameChar(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
}

func (s *scanner) scanSpaces() string {
	return s.substringToPos(s.findWsEnd(s.pos))
}

func (s *scanner) scanWord() string {
	end := s.pos
	for end < len(s.src) && isNameChar(s.src[end]) {
		end++
	}
	if s.pos == end {
		s.fail("Expect a keyword")
	}
	return s.substringToPos(end)
}

func (s *scanner) scanNumber() string {
	end := s.pos
	for end < len(s.src) && csstree.IsDigit(int(s.src[end])) {
		end++
	}
	if s.pos == end {
		s.fail("Expect a number")
	}
	return s.substringToPos(end)
}

// scanString reads an apostrophe-quoted literal including its quotes. Like
// css-tree it does not honour escapes: the literal ends at the next `'`.
func (s *scanner) scanString() string {
	end := slices.Index(s.src[s.pos+1:], '\'')
	if end == -1 {
		s.pos = len(s.src)
		s.fail("Expect an apostrophe")
	}
	return s.substringToPos(s.pos + 1 + end + 1)
}

// digitsValue converts a run of ASCII digits the way JavaScript's Number()
// does, so overly long runs round instead of failing.
func digitsValue(digits string) float64 {
	v, err := strconv.ParseFloat(digits, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		panic("defsyntax: scanned digits are not a number: " + digits)
	}
	return v
}

// toInt converts a multiplier bound. Bounds beyond the int range saturate,
// where css-tree would keep an inexact float.
func toInt(digits string) int {
	v := digitsValue(digits)
	if v >= math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

func (s *scanner) readMultiplierRange() (lo, hi int) {
	s.eat('{')
	minStr := s.scanNumber()
	maxStr := ""
	if s.charCode() == ',' {
		s.pos++
		if s.charCode() != '}' {
			maxStr = s.scanNumber()
		}
	} else {
		maxStr = minStr
	}
	s.eat('}')

	lo = toInt(minStr)
	if maxStr != "" {
		hi = toInt(maxStr)
	}
	return lo, hi
}

func (s *scanner) readMultiplier() *Multiplier {
	m := &Multiplier{}
	switch s.charCode() {
	case '*':
		s.pos++
		m.Min, m.Max = 0, 0
	case '+':
		s.pos++
		m.Min, m.Max = 1, 0
	case '?':
		s.pos++
		m.Min, m.Max = 0, 1
	case '#':
		s.pos++
		m.Comma = true
		switch s.charCode() {
		case '{':
			m.Min, m.Max = s.readMultiplierRange()
		case '?':
			// https://www.w3.org/TR/css-values-4/#component-multipliers
			// "the # and ? multipliers may be stacked as #?"; css-tree treats
			// "#?" as a single { min: 0, max: 0, comma: true } multiplier.
			s.pos++
			m.Min, m.Max = 0, 0
		default:
			m.Min, m.Max = 1, 0
		}
	case '{':
		m.Min, m.Max = s.readMultiplierRange()
	default:
		return nil
	}
	return m
}

func (s *scanner) maybeMultiplied(node Node) Node {
	m := s.readMultiplier()
	if m == nil {
		return node
	}
	m.Term = node

	// "The + and # multipliers may be stacked as +#": represented as a #
	// multiplier whose term is the + multiplier.
	if s.charCode() == '#' && s.charCodeAt(s.pos-1) == '+' {
		return s.maybeMultiplied(m)
	}
	return m
}

func (s *scanner) maybeToken() Node {
	ch := s.peekChar()
	if ch == "" {
		return nil
	}
	return &Token{Value: ch}
}

func (s *scanner) readProperty() Node {
	s.eat('<')
	s.eat('\'')
	name := s.scanWord()
	s.eat('\'')
	s.eat('>')
	return s.maybeMultiplied(&Property{Name: name})
}

// readRangeBound reads a finite bound after its optional sign. A unit
// following the number turns the bound into a dimension bound.
func (s *scanner) readRangeBound(sign float64) *RangeBound {
	b := &RangeBound{Value: sign * digitsValue(s.scanNumber())}
	if c := s.charCode(); c >= utf8.RuneSelf || isNameChar(c) {
		// css-tree tests NAME_CHAR[code] !== 0, which also holds for
		// non-ASCII characters; scanWord then rejects them.
		b.Unit = s.scanWord()
	}
	return b
}

// readTypeRange reads css-values bracketed range notation, e.g. the
// `[0,∞]` of `<integer [0,∞]>`.
func (s *scanner) readTypeRange() *Range {
	r := &Range{}
	sign := 1.0

	s.eat('[')
	if s.charCode() == '-' {
		s.peekChar()
		sign = -1
	}
	if sign == -1 && s.charCode() == infinity {
		s.peekChar()
	} else {
		r.Min = s.readRangeBound(sign)
	}

	s.scanSpaces()
	s.eat(',')
	s.scanSpaces()

	if s.charCode() == infinity {
		s.peekChar()
	} else {
		sign = 1
		if s.charCode() == '-' {
			s.peekChar()
			sign = -1
		}
		r.Max = s.readRangeBound(sign)
	}

	s.eat(']')
	return r
}

func (s *scanner) readType() Node {
	s.eat('<')
	name := s.scanWord()

	if s.charCode() == '(' && s.nextCharCode() == ')' {
		s.pos += 2
		name += "()"
	}

	var opts *Range
	if s.charCodeAt(s.findWsEnd(s.pos)) == '[' {
		s.scanSpaces()
		opts = s.readTypeRange()
	}

	s.eat('>')
	return s.maybeMultiplied(&Type{Name: name, Opts: opts})
}

func (s *scanner) readKeywordOrFunction() Node {
	name := s.scanWord()
	if s.charCode() == '(' {
		s.pos++
		return &Function{Name: name}
	}
	return s.maybeMultiplied(&Keyword{Name: name})
}

// regroupTerms folds the flat term list into nested groups, binding the
// tightest combinators first, and returns the loosest combinator present
// (the one the outer group uses), or "" when there is none.
func regroupTerms(terms []Node, combinators map[string]bool) ([]Node, string) {
	order := make([]string, 0, len(combinators))
	for c := range combinators {
		order = append(order, c)
	}
	slices.SortFunc(order, func(a, b string) int {
		return combinatorPrecedence[a] - combinatorPrecedence[b]
	})

	combinator := ""
	for len(order) > 0 {
		combinator, order = order[0], order[1:]

		i := 0
		subgroupStart := 0
		for ; i < len(terms); i++ {
			term, ok := terms[i].(*combinatorTerm)
			if !ok {
				continue
			}
			if term.value == combinator {
				if subgroupStart == -1 {
					subgroupStart = i - 1
				}
				terms = slices.Delete(terms, i, i+1)
				i--
			} else {
				if subgroupStart != -1 && i-subgroupStart > 1 {
					group := createGroup(slices.Clone(terms[subgroupStart:i]), combinator)
					terms = slices.Replace(terms, subgroupStart, i, Node(group))
					i = subgroupStart + 1
				}
				subgroupStart = -1
			}
		}

		if subgroupStart != -1 && len(order) > 0 {
			group := createGroup(slices.Clone(terms[subgroupStart:i]), combinator)
			terms = slices.Replace(terms, subgroupStart, i, Node(group))
		}
	}

	return terms, combinator
}

func createGroup(terms []Node, combinator string) *Group {
	return &Group{Terms: terms, Combinator: combinator}
}

func (s *scanner) readImplicitGroup() *Group {
	var terms []Node
	combinators := map[string]bool{}
	var prev Node
	prevPos := s.pos

	for {
		token := s.peek()
		if token == nil {
			break
		}
		if _, ok := token.(*spaces); ok {
			continue
		}
		if c, ok := token.(*combinatorTerm); ok {
			// a combinator may neither start a group nor follow another one
			if _, prevIsComb := prev.(*combinatorTerm); prev == nil || prevIsComb {
				s.pos = prevPos
				s.fail("Unexpected combinator")
			}
			combinators[c.value] = true
		} else if _, prevIsComb := prev.(*combinatorTerm); prev != nil && !prevIsComb {
			combinators[" "] = true
			terms = append(terms, &combinatorTerm{value: " "})
		}

		terms = append(terms, token)
		prev = token
		prevPos = s.pos
	}

	// a combinator may not end a group either
	if _, ok := prev.(*combinatorTerm); ok {
		// css-tree computes the reported offset as pos - prevTokenPos
		s.pos -= prevPos
		s.fail("Unexpected combinator")
	}

	terms, combinator := regroupTerms(terms, combinators)
	if combinator == "" {
		combinator = " "
	}
	return &Group{Terms: terms, Combinator: combinator}
}

func (s *scanner) readGroup() *Group {
	s.eat('[')
	result := s.readImplicitGroup()
	s.eat(']')

	result.Explicit = true
	if s.charCode() == '!' {
		s.pos++
		result.DisallowEmpty = true
	}
	return result
}

// peek reads the next term, combinator or whitespace run. It returns nil at
// the end of the input and at characters that end a group (`]` and the
// multiplier characters, which are only valid after a term).
func (s *scanner) peek() Node {
	code := s.charCode()
	if code < utf8.RuneSelf && isNameChar(code) {
		return s.readKeywordOrFunction()
	}

	switch code {
	case ']':
		// don't eat, stop scanning the group
		return nil
	case '[':
		return s.maybeMultiplied(s.readGroup())
	case '<':
		if s.nextCharCode() == '\'' {
			return s.readProperty()
		}
		return s.readType()
	case '|':
		width := 1
		if s.nextCharCode() == '|' {
			width = 2
		}
		return &combinatorTerm{value: s.substringToPos(s.pos + width)}
	case '&':
		s.pos++
		s.eat('&')
		return &combinatorTerm{value: "&&"}
	case ',':
		s.pos++
		return &Comma{}
	case '\'':
		return s.maybeMultiplied(&String{Value: s.scanString()})
	case ' ', '\t', '\n', '\r', '\f':
		s.scanSpaces()
		return &spaces{}
	case '@':
		if next := s.nextCharCode(); next < utf8.RuneSelf && isNameChar(next) {
			s.pos++
			return &AtKeyword{Name: s.scanWord()}
		}
		return s.maybeToken()
	case '*', '+', '?', '#', '!':
		// prohibited tokens (used as a multiplier start)
		return nil
	case '{':
		// `{` is allowed since mdn-data uses it unquoted, unless a digit
		// follows: then it is likely a detached multiplier
		if !csstree.IsDigit(int(s.nextCharCode())) {
			return s.maybeToken()
		}
		return nil
	default:
		return s.maybeToken()
	}
}

// Parse parses a value definition into its syntax tree. As in css-tree, a
// top-level group consisting of a single group is reduced to that group.
func Parse(source string) (node Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			syntaxErr, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			node, err = nil, syntaxErr
		}
	}()

	s := &scanner{src: []rune(source)}
	result := s.readImplicitGroup()
	if s.pos != len(s.src) {
		s.fail("Unexpected input")
	}

	if len(result.Terms) == 1 {
		if group, ok := result.Terms[0].(*Group); ok {
			return group, nil
		}
	}
	return result, nil
}
