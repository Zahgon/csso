package csstree

// This file ports the microsyntax parsers of css-tree: AnPlusB.js and
// UnicodeRange.js.

// checkInteger verifies that the current token, from offset on, is an
// integer (optionally signed).
func (p *parser) checkInteger(offset int, disallowSign bool) {
	pos := p.tokenStart + offset
	code := p.charCodeAt(pos)

	if code == '+' || code == '-' {
		if disallowSign {
			p.fail("Number sign is not allowed")
		}
		pos++
	}

	for ; pos < p.tokenEnd; pos++ {
		if !IsDigit(p.charCodeAt(pos)) {
			p.failAt("Integer is expected", pos)
		}
	}
}

func (p *parser) checkTokenIsInteger(disallowSign bool) {
	p.checkInteger(0, disallowSign)
}

func (p *parser) expectCharCode(offset, code int) {
	if p.cmpChar(p.tokenStart+offset, code) {
		return
	}
	msg := ""
	switch code {
	case 'n':
		msg = "N is expected"
	case '-':
		msg = "HyphenMinus is expected"
	}
	p.failAt(msg, p.tokenStart+offset)
}

// consumeB parses the `... <signed-integer>` or
// `... ['+' | '-'] <signless-integer>` tail of An+B; it returns "" (null)
// when there is no B part.
func (p *parser) consumeB() string {
	offset := 0
	sign := 0
	t := p.tokenType

	for t == WhiteSpaceToken || t == CommentToken {
		offset++
		t = p.lookupType(offset)
	}

	if t != NumberToken {
		if !p.isDelimAt('+', offset) && !p.isDelimAt('-', offset) {
			return ""
		}
		sign = '-'
		if p.isDelimAt('+', offset) {
			sign = '+'
		}
		for {
			offset++
			t = p.lookupType(offset)
			if t != WhiteSpaceToken && t != CommentToken {
				break
			}
		}
		if t != NumberToken {
			p.skip(offset)
			p.checkTokenIsInteger(true)
		}
	}

	if offset > 0 {
		p.skip(offset)
	}

	if sign == 0 {
		code := p.charCodeAt(p.tokenStart)
		if code != '+' && code != '-' {
			p.fail("Number sign is expected")
		}
	}

	p.checkTokenIsInteger(sign != 0)
	if sign == '-' {
		return "-" + p.consume(NumberToken)
	}
	return p.consume(NumberToken)
}

// parseAnPlusB parses the An+B microsyntax
// (https://www.w3.org/TR/css-syntax-3/#anb).
func (p *parser) parseAnPlusB() *AnPlusB {
	start := p.tokenStart
	a, b := "", ""

	switch {
	// <integer>
	case p.tokenType == NumberToken:
		p.checkTokenIsInteger(false)
		b = p.consume(NumberToken)

	// -n, -n <signed-integer>, -n ['+' | '-'] <signless-integer>,
	// -n- <signless-integer>, <dashndashdigit-ident>
	case p.tokenType == IdentToken && p.cmpChar(p.tokenStart, '-'):
		a = "-1"
		p.expectCharCode(1, 'n')

		switch p.tokenEnd - p.tokenStart {
		case 2:
			p.next()
			b = p.consumeB()
		case 3:
			p.expectCharCode(2, '-')
			p.next()
			p.skipSC()
			p.checkTokenIsInteger(true)
			b = "-" + p.consume(NumberToken)
		default:
			p.expectCharCode(2, '-')
			p.checkInteger(3, true)
			p.next()
			b = p.substrToCursor(start + 2)
		}

	// '+'? n, '+'? n <signed-integer>, '+'? n ['+' | '-'] <signless-integer>,
	// '+'? n- <signless-integer>, '+'? <ndashdigit-ident>
	case p.tokenType == IdentToken || (p.isDelim('+') && p.lookupType(1) == IdentToken):
		sign := 0
		a = "1"

		if p.isDelim('+') {
			sign = 1
			p.next()
		}

		p.expectCharCode(0, 'n')

		switch p.tokenEnd - p.tokenStart {
		case 1:
			p.next()
			b = p.consumeB()
		case 2:
			p.expectCharCode(1, '-')
			p.next()
			p.skipSC()
			p.checkTokenIsInteger(true)
			b = "-" + p.consume(NumberToken)
		default:
			p.expectCharCode(1, '-')
			p.checkInteger(2, true)
			p.next()
			b = p.substrToCursor(start + sign + 1)
		}

	// <ndashdigit-dimension>, <ndash-dimension> <signless-integer>,
	// <n-dimension>, <n-dimension> <signed-integer>,
	// <n-dimension> ['+' | '-'] <signless-integer>
	case p.tokenType == DimensionToken:
		code := p.charCodeAt(p.tokenStart)
		sign := 0
		if code == '+' || code == '-' {
			sign = 1
		}
		i := p.tokenStart + sign
		for ; i < p.tokenEnd; i++ {
			if !IsDigit(p.charCodeAt(i)) {
				break
			}
		}
		if i == p.tokenStart+sign {
			p.failAt("Integer is expected", p.tokenStart+sign)
		}

		p.expectCharCode(i-p.tokenStart, 'n')
		a = p.substring(start, i)

		if i+1 == p.tokenEnd {
			p.next()
			b = p.consumeB()
		} else {
			p.expectCharCode(i-p.tokenStart+1, '-')
			if i+2 == p.tokenEnd {
				p.next()
				p.skipSC()
				p.checkTokenIsInteger(true)
				b = "-" + p.consume(NumberToken)
			} else {
				p.checkInteger(i-p.tokenStart+2, true)
				p.next()
				b = p.substrToCursor(i + 1)
			}
		}

	default:
		p.fail("")
	}

	if len(a) > 0 && a[0] == '+' {
		a = a[1:]
	}
	if len(b) > 0 && b[0] == '+' {
		b = b[1:]
	}

	return &AnPlusB{Base: p.base(start, p.tokenStart), A: a, B: b}
}

// eatHexSequence consumes up to six hex digits of the current token from
// offset on; with allowDash a `-` splits a range and the second half is
// checked too (the result is then -1).
func (p *parser) eatHexSequence(offset int, allowDash bool) int {
	length := 0

	for pos := p.tokenStart + offset; pos < p.tokenEnd; pos++ {
		code := p.charCodeAt(pos)

		if code == '-' && allowDash && length != 0 {
			p.eatHexSequence(offset+length+1, false)
			return -1
		}

		if !IsHexDigit(code) {
			var msg string
			switch {
			case allowDash && length != 0 && length < 6:
				msg = "Hyphen minus or hex digit is expected"
			case allowDash && length != 0:
				msg = "Hyphen minus is expected"
			case length < 6:
				msg = "Hex digit is expected"
			default:
				msg = "Unexpected input"
			}
			p.failAt(msg, pos)
		}

		length++
		if length > 6 {
			p.failAt("Too many hex digits", pos)
		}
	}

	p.next()
	return length
}

func (p *parser) eatQuestionMarkSequence(max int) {
	count := 0
	for p.isDelim('?') {
		count++
		if count > max {
			p.fail("Too many question marks")
		}
		p.next()
	}
}

func (p *parser) unicodeRangeStartsWith(code int) {
	if p.charCodeAt(p.tokenStart) == code {
		return
	}
	if code == '+' {
		p.fail("Plus sign is expected")
	}
	p.fail("Hyphen minus is expected")
}

// scanUnicodeRange consumes the tokens of a <urange> after the leading `u`
// (https://drafts.csswg.org/css-syntax/#urange):
//
//	u '+' <ident-token> '?'* |
//	u <dimension-token> '?'* |
//	u <number-token> '?'* |
//	u <number-token> <dimension-token> |
//	u <number-token> <number-token> |
//	u '+' '?'+
func (p *parser) scanUnicodeRange() {
	switch p.tokenType {
	case NumberToken:
		hexLength := p.eatHexSequence(1, true)
		if p.isDelim('?') {
			p.eatQuestionMarkSequence(6 - hexLength)
			return
		}
		if p.tokenType == DimensionToken || p.tokenType == NumberToken {
			p.unicodeRangeStartsWith('-')
			p.eatHexSequence(1, false)
		}

	case DimensionToken:
		hexLength := p.eatHexSequence(1, true)
		if hexLength > 0 {
			p.eatQuestionMarkSequence(6 - hexLength)
		}

	default:
		p.eatDelim('+')
		if p.tokenType == IdentToken {
			hexLength := p.eatHexSequence(0, true)
			if hexLength > 0 {
				p.eatQuestionMarkSequence(6 - hexLength)
			}
			return
		}
		if p.isDelim('?') {
			p.next()
			p.eatQuestionMarkSequence(5)
			return
		}
		p.fail("Hex digit or question mark is expected")
	}
}

func (p *parser) parseUnicodeRange() *UnicodeRange {
	start := p.tokenStart
	p.eatIdent("u")
	p.scanUnicodeRange()
	return &UnicodeRange{Base: p.base(start, p.tokenStart), Value: p.substrToCursor(start)}
}
