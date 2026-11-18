package lexer

import (
	"fmt"
	"strings"

	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// iterationLimit bounds the matcher's state transitions, as css-tree's
// ITERATION_LIMIT does; a match that needs more fails.
const iterationLimit = 15000

// Exit reasons of a match attempt, as css-tree reports them.
const (
	reasonMatch          = "Match"
	reasonMismatch       = "Mismatch"
	reasonIterationLimit = "Maximum iteration number exceeded (please fill an issue on https://github.com/csstree/csstree/issues)"
)

// token is a value token as the matcher sees it, attributed to the AST node
// whose generation emitted it (nil for tokens the generator inserts itself).
type token struct {
	typ   csstree.TokenType
	value string
	node  csstree.Node
}

// matchItemKind tells what a matchItem records.
type matchItemKind uint8

const (
	itemStub matchItemKind = iota
	itemToken
	itemOpenSyntax
	itemCloseSyntax
)

// matchItem is an entry of the match stack, a persistent linked list that
// backtracking restores by pointer.
type matchItem struct {
	kind   matchItemKind
	syntax defsyntax.Node
	token  *token
	prev   *matchItem
}

// syntaxFrame is an entry of the stack of opened Type/Property syntaxes.
type syntaxFrame struct {
	syntax defsyntax.Node
	opts   *defsyntax.Range
	prev   *syntaxFrame
}

// thenFrame is a continuation to run once the current state matches.
type thenFrame struct {
	next        *state
	matchStack  *matchItem
	syntaxStack *syntaxFrame
	prev        *thenFrame
}

// elseFrame is a backtracking point: the state to try when matching fails,
// with everything needed to restore the matcher.
type elseFrame struct {
	next        *state
	matchStack  *matchItem
	syntaxStack *syntaxFrame
	thenStack   *thenFrame
	tokenIndex  int
	prev        *elseFrame
}

// matchOutcome is the result of internalMatch.
type matchOutcome struct {
	tokens       []token
	reason       string
	iterations   int
	match        *matchItem // nil unless reason is reasonMatch
	longestMatch int
}

// isContextEdgeDelim excludes `?` so that unicode ranges such as U+30?? do
// not start a comma context.
func isContextEdgeDelim(tok *token) bool {
	return tok.typ == csstree.DelimToken && tok.value != "?"
}

// isCommaContextStart reports whether a comma may be omitted (or must not
// appear) after tok, the last matched token.
func isCommaContextStart(tok *token) bool {
	if tok == nil {
		return true
	}
	switch tok.typ {
	case csstree.CommaToken, csstree.FunctionToken, csstree.LeftParenthesisToken,
		csstree.LeftSquareBracketToken, csstree.LeftCurlyBracketToken:
		return true
	}
	return isContextEdgeDelim(tok)
}

// isCommaContextEnd reports whether a comma may be omitted (or must not
// appear) before tok, the next token.
func isCommaContextEnd(tok *token) bool {
	if tok == nil {
		return true
	}
	switch tok.typ {
	case csstree.RightParenthesisToken, csstree.RightSquareBracketToken, csstree.RightCurlyBracketToken:
		return true
	}
	return tok.typ == csstree.DelimToken && tok.value == "/"
}

// stripIEHack drops a `\0` or `\9` hack and everything after it from a
// keyword, like css-tree's name.replace(/\\[09].*$/, ""): the hack must be
// followed by no line terminator up to the end of the name.
func stripIEHack(name string) string {
	for i := 0; i+1 < len(name); i++ {
		if name[i] == '\\' && (name[i+1] == '0' || name[i+1] == '9') && !strings.ContainsAny(name[i+2:], "\n\r\u2028\u2029") {
			return name[:i]
		}
	}
	return name
}

// internalMatch runs css-tree's backtracking matcher over tokens starting at
// the graph root, resolving Type and Property references with the lexer's
// dictionaries. It fails with an error only for a reference the lexer does
// not know (css-tree throws "Bad syntax reference").
func internalMatch(tokens []token, root *state, l *Lexer) (*matchOutcome, error) {
	var (
		syntaxStack *syntaxFrame
		thenStack   *thenFrame
		elseStack   *elseFrame

		// stash holds a deferred low-priority syntax (custom-ident, or
		// length for a `0`); stashDisabled is css-tree's syntaxStash ===
		// false, set once a stash has been restored.
		stash         *elseFrame
		stashDisabled bool

		iterationCount int
		exitReason     string

		tok          *token
		tokenIndex   = -1
		longestMatch = 0
		matchStack   = &matchItem{kind: itemStub}
	)

	tokenAt := func(i int) *token {
		if i < len(tokens) {
			return &tokens[i]
		}
		return nil
	}

	moveToNextToken := func() {
		for {
			tokenIndex++
			tok = tokenAt(tokenIndex)
			if tok == nil || tok.typ != csstree.WhiteSpaceToken && tok.typ != csstree.CommentToken {
				return
			}
		}
	}

	getNextToken := func(offset int) *token {
		return tokenAt(tokenIndex + offset)
	}

	snapshot := func(next *state, prev *elseFrame) *elseFrame {
		return &elseFrame{
			next:        next,
			matchStack:  matchStack,
			syntaxStack: syntaxStack,
			thenStack:   thenStack,
			tokenIndex:  tokenIndex,
			prev:        prev,
		}
	}

	pushThenStack := func(next *state) {
		thenStack = &thenFrame{next: next, matchStack: matchStack, syntaxStack: syntaxStack, prev: thenStack}
	}

	pushElseStack := func(next *state) {
		elseStack = snapshot(next, elseStack)
	}

	addTokenToMatch := func(s *state) {
		matchStack = &matchItem{kind: itemToken, syntax: s.syntax, token: tok, prev: matchStack}
		moveToNextToken()
		stash, stashDisabled = nil, false
		if tokenIndex > longestMatch {
			longestMatch = tokenIndex
		}
	}

	openSyntax := func(s *state) {
		var opts *defsyntax.Range
		if t, ok := s.syntax.(*defsyntax.Type); ok {
			opts = t.Opts
		}
		if opts == nil && syntaxStack != nil {
			opts = syntaxStack.opts
		}
		syntaxStack = &syntaxFrame{syntax: s.syntax, opts: opts, prev: syntaxStack}
		matchStack = &matchItem{kind: itemOpenSyntax, syntax: s.syntax, token: matchStack.token, prev: matchStack}
	}

	closeSyntax := func() {
		if matchStack.kind == itemOpenSyntax {
			matchStack = matchStack.prev
		} else {
			matchStack = &matchItem{kind: itemCloseSyntax, syntax: syntaxStack.syntax, token: matchStack.token, prev: matchStack}
		}
		syntaxStack = syntaxStack.prev
	}

	moveToNextToken()

	s := root
	for exitReason == "" {
		iterationCount++
		if iterationCount >= iterationLimit {
			break
		}

		switch s.kind {
		case stateMatch:
			if thenStack == nil {
				// turn to a mismatch when some tokens are left unmatched,
				// unless it is a single trailing IE hack (\0 or \9)
				if tok != nil && (tokenIndex != len(tokens)-1 || tok.value != `\0` && tok.value != `\9`) {
					s = mismatchState
					break
				}
				exitReason = reasonMatch
				break
			}

			// continue with the `then` branch
			s = thenStack.next

			// check the match is not empty
			if s == disallowEmptyState {
				if thenStack.matchStack == matchStack {
					s = mismatchState
					break
				}
				s = matchState
			}

			for thenStack.syntaxStack != syntaxStack {
				closeSyntax()
			}
			thenStack = thenStack.prev

		case stateMismatch:
			if stash != nil {
				// restore the stashed syntax when there is no else branch
				// or the else branch would rewind further
				if elseStack == nil || tokenIndex > elseStack.tokenIndex {
					elseStack = stash
					stash, stashDisabled = nil, true
				}
			} else if elseStack == nil {
				exitReason = reasonMismatch
				break
			}

			// continue with the `else` branch, restoring its state
			s = elseStack.next
			thenStack = elseStack.thenStack
			syntaxStack = elseStack.syntaxStack
			matchStack = elseStack.matchStack
			tokenIndex = elseStack.tokenIndex
			tok = tokenAt(tokenIndex)
			elseStack = elseStack.prev

		case stateMatchGraph:
			s = s.match

		case stateIf:
			// the else push goes first: it records thenStack before the
			// then push changes it
			if s.els != mismatchState {
				pushElseStack(s.els)
			}
			if s.then != matchState {
				pushThenStack(s.then)
			}
			s = s.match

		case stateMatchOnce:
			s = &state{kind: stateMatchOnceBuffer, once: s}

		case stateMatchOnceBuffer:
			terms := s.once.terms

			if s.index == len(terms) {
				// no match at all, or a term required by && is missing
				if s.mask == 0 || s.once.all {
					s = mismatchState
					break
				}
				// a partial match is fine for ||
				s = matchState
				break
			}

			// all terms are matched
			if s.mask == 1<<len(terms)-1 {
				s = matchState
				break
			}

			for ; s.index < len(terms); s.index++ {
				flag := 1 << s.index
				if s.mask&flag == 0 {
					// the buffer itself is the else branch: it resumes
					// from the next term, since index is advanced below
					pushElseStack(s)
					pushThenStack(&state{kind: stateAddMatchOnce, once: s.once, mask: s.mask | flag})
					term := terms[s.index]
					s.index++
					s = term
					break
				}
			}

		case stateAddMatchOnce:
			s = &state{kind: stateMatchOnceBuffer, once: s.once, mask: s.mask}

		case stateEnum:
			if tok != nil {
				name := toLowerCase(tok.value)
				if strings.Contains(name, `\`) {
					name = stripIEHack(name)
				}
				if alt, ok := s.enum[name]; ok {
					s = alt
					break
				}
			}
			s = mismatchState

		case stateGeneric:
			var opts *defsyntax.Range
			if syntaxStack != nil {
				opts = syntaxStack.opts
			}
			lastTokenIndex := tokenIndex + s.generic(tok, getNextToken, opts)
			if lastTokenIndex > tokenIndex {
				for tokenIndex < lastTokenIndex {
					addTokenToMatch(s)
				}
				s = matchState
			} else {
				s = mismatchState
			}

		case stateType, stateProperty:
			descriptor := l.properties[s.name]
			if s.kind == stateType {
				descriptor = l.types[s.name]
			}
			if descriptor == nil {
				ref := "<" + s.name + ">"
				if s.kind == stateProperty {
					ref = "<'" + s.name + "'>"
				}
				return nil, fmt.Errorf("lexer: bad syntax reference: %s", ref)
			}

			// stash a syntax of low priority
			if !stashDisabled && tok != nil && s.kind == stateType {
				lowPriority :=
					// https://drafts.csswg.org/css-values-4/#custom-idents: a
					// <custom-ident> can only claim a keyword no other
					// unfulfilled production can claim
					s.name == "custom-ident" && tok.typ == csstree.IdentToken ||
						// https://drafts.csswg.org/css-values-4/#lengths: a 0
						// that parses as either <number> or <length> is a
						// <number>
						s.name == "length" && tok.value == "0"

				if lowPriority {
					if stash == nil {
						stash = snapshot(s, elseStack)
					}
					s = mismatchState
					break
				}
			}

			openSyntax(s)
			s = descriptor.match

		case stateKeyword:
			if tok != nil {
				name := tok.value
				if strings.Contains(name, `\`) {
					name = stripIEHack(name)
				}
				if equalsLower(name, s.name) {
					addTokenToMatch(s)
					s = matchState
					break
				}
			}
			s = mismatchState

		case stateAtKeyword, stateFunction:
			if tok != nil && equalsLower(tok.value, s.name) {
				addTokenToMatch(s)
				s = matchState
				break
			}
			s = mismatchState

		case stateToken:
			if tok != nil && tok.value == s.value {
				addTokenToMatch(s)
				s = matchState
				break
			}
			s = mismatchState

		case stateComma:
			if tok != nil && tok.typ == csstree.CommaToken {
				if isCommaContextStart(matchStack.token) {
					s = mismatchState
				} else {
					addTokenToMatch(s)
					if isCommaContextEnd(tok) {
						s = mismatchState
					} else {
						s = matchState
					}
				}
			} else if isCommaContextStart(matchStack.token) || isCommaContextEnd(tok) {
				s = matchState
			} else {
				s = mismatchState
			}

		case stateString:
			// concatenate tokens until the literal's length is reached; the
			// literals are ASCII, so byte lengths decide like UTF-16 ones
			text := ""
			lastTokenIndex := tokenIndex
			for ; lastTokenIndex < len(tokens) && len(text) < len(s.value); lastTokenIndex++ {
				text += tokens[lastTokenIndex].value
			}
			if equalsLower(text, s.value) {
				for tokenIndex < lastTokenIndex {
					addTokenToMatch(s)
				}
				s = matchState
			} else {
				s = mismatchState
			}

		default:
			panic(fmt.Sprintf("lexer: unknown match state kind %d", s.kind))
		}
	}

	switch exitReason {
	case "":
		exitReason = reasonIterationLimit
		matchStack = nil
	case reasonMatch:
		for syntaxStack != nil {
			closeSyntax()
		}
	default:
		matchStack = nil
	}

	return &matchOutcome{
		tokens:       tokens,
		reason:       exitReason,
		iterations:   iterationCount,
		match:        matchStack,
		longestMatch: longestMatch,
	}, nil
}
