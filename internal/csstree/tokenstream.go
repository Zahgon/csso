package csstree

// tokenStream is css-tree's TokenStream: the token list of a source plus a
// balance table pairing every opening bracket/function token with its
// closing token, and a cursor over the tokens.
type tokenStream struct {
	source []uint16

	types   []TokenType // token types; types[tokenCount] is EOF
	ends    []int       // token end offsets; ends[tokenCount] is len(source)
	balance []int       // see setSource

	tokenCount      int
	firstCharOffset int

	eof        bool
	tokenIndex int
	tokenType  TokenType
	tokenStart int
	tokenEnd   int
}

// closingType returns the token type that closes an opening token, or
// EOFToken for tokens that open nothing.
func closingType(t TokenType) TokenType {
	switch t {
	case FunctionToken, LeftParenthesisToken:
		return RightParenthesisToken
	case LeftSquareBracketToken:
		return RightSquareBracketToken
	case LeftCurlyBracketToken:
		return RightCurlyBracketToken
	}
	return EOFToken
}

// setSource tokenizes source and builds the balance table: for an opening
// token it holds the index of its closing token, for a closing token the
// index of its opening token, and for any other token the index of the
// closing token of the innermost enclosing block (len(source) when there is
// none or the block is never closed).
func (ts *tokenStream) setSource(source []uint16) {
	sourceLength := len(source)
	types := make([]TokenType, 0, sourceLength/4+1)
	ends := make([]int, 0, sourceLength/4+1)
	balance := make([]int, 0, sourceLength/4+1)
	type opener struct {
		index      int
		closeType  TokenType
		outerIndex int
	}
	var stack []opener
	closeType := EOFToken
	firstCharOffset := -1

	tokenizeUnits(source, func(t TokenType, start, end int) {
		tokenCount := len(types)
		switch {
		case t == closeType:
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			closeType = EOFToken
			if len(stack) > 0 {
				closeType = stack[len(stack)-1].closeType
			}
			balance = append(balance, open.index)
			balance[open.index] = tokenCount
			for i := open.index + 1; i < tokenCount; i++ {
				if balance[i] == sourceLength {
					balance[i] = tokenCount
				}
			}
		case closingType(t) != EOFToken:
			balance = append(balance, 0)
			closeType = closingType(t)
			stack = append(stack, opener{index: tokenCount, closeType: closeType})
		default:
			balance = append(balance, sourceLength)
		}
		types = append(types, t)
		ends = append(ends, end)
		if firstCharOffset == -1 {
			firstCharOffset = start
		}
	})

	tokenCount := len(types)
	types = append(types, EOFToken)
	ends = append(ends, sourceLength)
	balance = append(balance, sourceLength)
	for _, open := range stack {
		balance[open.index] = sourceLength
	}

	ts.source = source
	ts.types = types
	ts.ends = ends
	ts.balance = balance
	ts.tokenCount = tokenCount
	ts.firstCharOffset = max(firstCharOffset, 0)
	ts.reset()
	ts.next()
}

// balanceAt reads the balance table; css-tree pins index len(source) to
// len(source) so lookups through an unclosed block land on a sentinel.
func (ts *tokenStream) balanceAt(i int) int {
	if i < len(ts.balance) {
		return ts.balance[i]
	}
	return len(ts.source)
}

func (ts *tokenStream) reset() {
	ts.eof = false
	ts.tokenIndex = -1
	ts.tokenType = EOFToken
	ts.tokenStart = ts.firstCharOffset
	ts.tokenEnd = ts.firstCharOffset
}

// endOf returns the end offset of token i; css-tree reads index -1 of its
// typed array as undefined, which masks to 0.
func (ts *tokenStream) endOf(i int) int {
	if i < 0 {
		return 0
	}
	return ts.ends[i]
}

func (ts *tokenStream) lookupType(offset int) TokenType {
	offset += ts.tokenIndex
	if offset < ts.tokenCount {
		return ts.types[offset]
	}
	return EOFToken
}

func (ts *tokenStream) lookupOffset(offset int) int {
	offset += ts.tokenIndex
	if offset < ts.tokenCount {
		return ts.endOf(offset - 1)
	}
	return len(ts.source)
}

func (ts *tokenStream) lookupValue(offset int, reference string) bool {
	offset += ts.tokenIndex
	if offset < ts.tokenCount {
		return cmpStr(ts.source, ts.endOf(offset-1), ts.ends[offset], reference)
	}
	return false
}

func (ts *tokenStream) getTokenStart(tokenIndex int) int {
	if tokenIndex == ts.tokenIndex {
		return ts.tokenStart
	}
	if tokenIndex > 0 {
		if tokenIndex < ts.tokenCount {
			return ts.ends[tokenIndex-1]
		}
		return ts.ends[ts.tokenCount]
	}
	return ts.firstCharOffset
}

func (ts *tokenStream) isBalanceEdge(pos int) bool {
	return ts.balanceAt(ts.tokenIndex) < pos
}

// isDelim reports whether the current token is a delim with the given code.
func (ts *tokenStream) isDelim(code int) bool {
	return ts.tokenType == DelimToken && unitAt(ts.source, ts.tokenStart) == code
}

// isDelimAt is isDelim for the token at a relative offset.
func (ts *tokenStream) isDelimAt(code, offset int) bool {
	if offset == 0 {
		return ts.isDelim(code)
	}
	return ts.lookupType(offset) == DelimToken && unitAt(ts.source, ts.lookupOffset(offset)) == code
}

func (ts *tokenStream) skip(tokenCount int) {
	next := ts.tokenIndex + tokenCount
	if next < ts.tokenCount {
		ts.tokenIndex = next
		ts.tokenStart = ts.endOf(next - 1)
		ts.tokenType = ts.types[next]
		ts.tokenEnd = ts.ends[next]
	} else {
		ts.tokenIndex = ts.tokenCount
		ts.next()
	}
}

func (ts *tokenStream) next() {
	next := ts.tokenIndex + 1
	if next < ts.tokenCount {
		ts.tokenIndex = next
		ts.tokenStart = ts.tokenEnd
		ts.tokenType = ts.types[next]
		ts.tokenEnd = ts.ends[next]
	} else {
		ts.eof = true
		ts.tokenIndex = ts.tokenCount
		ts.tokenType = EOFToken
		ts.tokenStart = len(ts.source)
		ts.tokenEnd = len(ts.source)
	}
}

func (ts *tokenStream) skipSC() {
	for ts.tokenType == WhiteSpaceToken || ts.tokenType == CommentToken {
		ts.next()
	}
}

// Stop conditions returned by a consumeUntil function of skipUntilBalanced.
const (
	scanContinue = iota
	scanStop
	scanStopIncluded
)

// skipUntilBalanced advances from startToken until stopConsume asks to stop
// at a token's first code unit, jumping over balanced blocks, or until the
// end of the block enclosing startToken.
func (ts *tokenStream) skipUntilBalanced(startToken int, stopConsume func(code int) int) {
	cursor := startToken
loop:
	for ; cursor < ts.tokenCount; cursor++ {
		balanceEnd := ts.balance[cursor]
		if balanceEnd < startToken {
			break
		}
		offset := ts.firstCharOffset
		if cursor > 0 {
			offset = ts.ends[cursor-1]
		}
		switch stopConsume(unitAt(ts.source, offset)) {
		case scanStop:
			break loop
		case scanStopIncluded:
			cursor++
			break loop
		default:
			if ts.balanceAt(balanceEnd) == cursor {
				cursor = balanceEnd
			}
		}
	}
	ts.skip(cursor - ts.tokenIndex)
}

// forEachToken calls fn for every token with its type and offsets.
func (ts *tokenStream) forEachToken(fn func(t TokenType, start, end int)) {
	offset := ts.firstCharOffset
	for i := 0; i < ts.tokenCount; i++ {
		start := offset
		offset = ts.ends[i]
		fn(ts.types[i], start, offset)
	}
}
