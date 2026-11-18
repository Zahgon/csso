package lexer

import (
	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// The <an-plus-b> microsyntax, https://drafts.csswg.org/css-syntax/#anb-microsyntax,
// matched over tokens as css-tree's generic-an-plus-b.js does.

func isDelim(tok *token, code byte) bool {
	return tok != nil && tok.typ == csstree.DelimToken && charAt(tok.value, 0) == int(code)
}

// cmpChar reports whether the character at offset is the lower-case
// reference, folding A-Z.
func cmpChar(s string, offset int, reference byte) bool {
	c := charAt(s, offset)
	if c >= 'A' && c <= 'Z' {
		c |= 0x20
	}
	return c == int(reference)
}

// skipSC returns the offset of the first token at or after offset that is
// neither whitespace nor a comment.
func skipSC(tok *token, offset int, next func(int) *token) int {
	for tok != nil && (tok.typ == csstree.WhiteSpaceToken || tok.typ == csstree.CommentToken) {
		offset++
		tok = next(offset)
	}
	return offset
}

// checkInteger checks that tok holds an integer from valueOffset on and
// returns the consumed token count (offset + 1), or 0.
func checkInteger(tok *token, valueOffset int, disallowSign bool, offset int) int {
	if tok == nil {
		return 0
	}

	if c := charAt(tok.value, valueOffset); c == '+' || c == '-' {
		if disallowSign {
			return 0
		}
		valueOffset++
	}

	for ; valueOffset < len(tok.value); valueOffset++ {
		if !csstree.IsDigit(int(tok.value[valueOffset])) {
			return 0
		}
	}

	return offset + 1
}

// consumeB matches the optional `['+' | '-'] <signless-integer>` or
// <signed-integer> part that follows `n`. Without a B it returns the token
// count consumed so far.
func consumeB(tok *token, start int, next func(int) *token) int {
	sign := false
	offset := skipSC(tok, start, next)

	tok = next(offset)
	if tok == nil {
		return start
	}

	if tok.typ != csstree.NumberToken {
		if !isDelim(tok, '+') && !isDelim(tok, '-') {
			return start
		}
		sign = true
		offset++
		offset = skipSC(next(offset), offset, next)
		tok = next(offset)
		if tok == nil || tok.typ != csstree.NumberToken {
			return 0
		}
	}

	if !sign {
		if c := charAt(tok.value, 0); c != '+' && c != '-' {
			return 0
		}
	}

	valueOffset := 1
	if sign {
		valueOffset = 0
	}
	return checkInteger(tok, valueOffset, sign, offset)
}

// matchNDashB matches the rest of `n-` or `-n-` forms: a dash ending the
// identifier is followed by a separate signless integer, otherwise the digits
// follow the dash inside the identifier itself (at bOffset).
func matchNDashB(tok *token, dashEnd bool, bOffset int, next func(int) *token) int {
	if dashEnd {
		offset := skipSC(next(1), 1, next)
		return checkInteger(next(offset), 0, true, offset)
	}
	return checkInteger(tok, bOffset, true, 0)
}

func anPlusB(tok *token, next func(int) *token, _ *defsyntax.Range) int {
	if tok == nil {
		return 0
	}

	switch {
	case tok.typ == csstree.NumberToken:
		// <integer>: B alone
		return checkInteger(tok, 0, false, 0)

	case tok.typ == csstree.IdentToken && charAt(tok.value, 0) == '-':
		// -n, -n-, -n-B, -n ['+' | '-'] B
		if !cmpChar(tok.value, 1, 'n') {
			return 0
		}
		switch len(tok.value) {
		case 2:
			return consumeB(next(1), 1, next)
		case 3:
			if charAt(tok.value, 2) != '-' {
				return 0
			}
			return matchNDashB(tok, true, 0, next)
		default:
			if charAt(tok.value, 2) != '-' {
				return 0
			}
			return matchNDashB(tok, false, 3, next)
		}

	case tok.typ == csstree.IdentToken || isDelim(tok, '+') && isIdentAt(next, 1):
		// n, +n, n-, n-B, n ['+' | '-'] B
		offset := 0
		if tok.typ != csstree.IdentToken {
			offset = 1
			tok = next(offset)
		}
		if tok == nil || !cmpChar(tok.value, 0, 'n') {
			return 0
		}
		switch len(tok.value) {
		case 1:
			return consumeB(next(offset+1), offset+1, next)
		case 2:
			if charAt(tok.value, 1) != '-' {
				return 0
			}
			offset = skipSC(next(offset+1), offset+1, next)
			return checkInteger(next(offset), 0, true, offset)
		default:
			if charAt(tok.value, 1) != '-' {
				return 0
			}
			return checkInteger(tok, 2, true, offset)
		}

	case tok.typ == csstree.DimensionToken:
		// <n-dimension>, <ndash-dimension>, <ndashdigit-dimension>
		sign := 0
		if c := charAt(tok.value, 0); c == '+' || c == '-' {
			sign = 1
		}
		i := decimalEnd(tok.value, sign)
		if i == sign || !cmpChar(tok.value, i, 'n') {
			return 0
		}
		if i+1 == len(tok.value) {
			return consumeB(next(1), 1, next)
		}
		if charAt(tok.value, i+1) != '-' {
			return 0
		}
		return matchNDashB(tok, i+2 == len(tok.value), i+2, next)
	}

	return 0
}

// isIdentAt reports whether the token at offset is an identifier. css-tree
// dereferences the token unchecked here and throws a TypeError past the end
// of the input; that is reported as no match instead.
func isIdentAt(next func(int) *token, offset int) bool {
	tok := next(offset)
	return tok != nil && tok.typ == csstree.IdentToken
}
