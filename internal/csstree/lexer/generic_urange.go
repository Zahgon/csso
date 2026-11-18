package lexer

import (
	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// The <urange> production, https://drafts.csswg.org/css-syntax/#urange,
// matched over the tokens `u+0-7F` and friends tokenize into, as css-tree's
// generic-urange.js does.

// hexSequence counts the hex digits of tok.value from offset on: 0 when a
// non-hex character or more than 6 digits occur. When allowDash is set, a dash
// after at least one digit ends the sequence and yields 6, which forbids
// trailing question marks.
func hexSequence(tok *token, offset int, allowDash bool) int {
	hexLen := 0

	for pos := offset; pos < len(tok.value); pos++ {
		c := int(tok.value[pos])

		if c == '-' && allowDash && hexLen != 0 {
			// css-tree scans the part after the dash but ignores the result
			return 6
		}
		if !csstree.IsHexDigit(c) {
			return 0
		}
		hexLen++
		if hexLen > 6 {
			return 0
		}
	}

	return hexLen
}

// withQuestionMarkSequence extends a match of length tokens holding consumed
// hex digits by the `?` delimiters that follow, up to 6 characters in total.
func withQuestionMarkSequence(consumed, length int, next func(int) *token) int {
	if consumed == 0 {
		return 0
	}

	for isDelim(next(length), '?') {
		consumed++
		if consumed > 6 {
			return 0
		}
		length++
	}

	return length
}

// urange matches `u` followed by:
//
//	'+' <ident-token> '?'*
//	<dimension-token> '?'*
//	<number-token> '?'*
//	<number-token> <dimension-token>
//	<number-token> <number-token>
//	'+' '?'+
func urange(tok *token, next func(int) *token, _ *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.IdentToken || !cmpChar(tok.value, 0, 'u') {
		return 0
	}

	length := 1
	tok = next(length)
	if tok == nil {
		return 0
	}

	if isDelim(tok, '+') {
		length++
		tok = next(length)
		if tok == nil {
			return 0
		}
		if tok.typ == csstree.IdentToken {
			return withQuestionMarkSequence(hexSequence(tok, 0, true), length+1, next)
		}
		if isDelim(tok, '?') {
			return withQuestionMarkSequence(1, length+1, next)
		}
		return 0
	}

	if tok.typ == csstree.NumberToken {
		consumedHexLen := hexSequence(tok, 1, true)
		if consumedHexLen == 0 {
			return 0
		}

		length++
		tok = next(length)
		if tok == nil {
			return length
		}

		if tok.typ == csstree.DimensionToken || tok.typ == csstree.NumberToken {
			if charAt(tok.value, 0) != '-' || hexSequence(tok, 1, false) == 0 {
				return 0
			}
			return length + 1
		}

		return withQuestionMarkSequence(consumedHexLen, length, next)
	}

	if tok.typ == csstree.DimensionToken {
		return withQuestionMarkSequence(hexSequence(tok, 1, true), length+1, next)
	}

	return 0
}
