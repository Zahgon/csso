package csstree

// TokenType is a CSS Syntax Level 3 token type, numbered as in css-tree's
// tokenizer/types.js.
type TokenType int

// Token types (css-tree tokenizer/types.js).
const (
	EOFToken                TokenType = iota // <EOF-token>
	IdentToken                               // <ident-token>
	FunctionToken                            // <function-token>
	AtKeywordToken                           // <at-keyword-token>
	HashToken                                // <hash-token>
	StringToken                              // <string-token>
	BadStringToken                           // <bad-string-token>
	URLToken                                 // <url-token>
	BadURLToken                              // <bad-url-token>
	DelimToken                               // <delim-token>
	NumberToken                              // <number-token>
	PercentageToken                          // <percentage-token>
	DimensionToken                           // <dimension-token>
	WhiteSpaceToken                          // <whitespace-token>
	CDOToken                                 // <CDO-token>
	CDCToken                                 // <CDC-token>
	ColonToken                               // <colon-token>
	SemicolonToken                           // <semicolon-token>
	CommaToken                               // <comma-token>
	LeftSquareBracketToken                   // <[-token>
	RightSquareBracketToken                  // <]-token>
	LeftParenthesisToken                     // <(-token>
	RightParenthesisToken                    // <)-token>
	LeftCurlyBracketToken                    // <{-token>
	RightCurlyBracketToken                   // <}-token>
	CommentToken                             // comment (not a spec token)
)

var tokenNames = [...]string{
	"EOF-token",
	"ident-token",
	"function-token",
	"at-keyword-token",
	"hash-token",
	"string-token",
	"bad-string-token",
	"url-token",
	"bad-url-token",
	"delim-token",
	"number-token",
	"percentage-token",
	"dimension-token",
	"whitespace-token",
	"CDO-token",
	"CDC-token",
	"colon-token",
	"semicolon-token",
	"comma-token",
	"[-token",
	"]-token",
	"(-token",
	")-token",
	"{-token",
	"}-token",
}

// String returns css-tree's token name (tokenNames), e.g. "ident-token".
// The comment type has no name in css-tree and yields "comment".
func (t TokenType) String() string {
	if t >= 0 && int(t) < len(tokenNames) {
		return tokenNames[t]
	}
	if t == CommentToken {
		return "comment"
	}
	return "unknown-token"
}

// Token is a token produced by Tokenize. Start and End are offsets in UTF-16
// code units, as in css-tree.
type Token struct {
	Type  TokenType
	Value string
	Start int
	End   int
}

// Tokenize splits source into tokens the way css-tree's tokenize() does.
// A leading byte order mark is skipped. Like css-tree, a bad URL ending in
// an escape at the end of input reports an End one past the source length.
func Tokenize(source string) []Token {
	units := toUnits(source)
	var tokens []Token
	tokenizeUnits(units, func(t TokenType, start, end int) {
		tokens = append(tokens, Token{Type: t, Value: sliceUnits(units, start, end), Start: start, End: end})
	})
	return tokens
}

// Character categories of css-tree's charCodeCategory(). Codes below 0x80
// without a named category are their own category.
const (
	eofCategory          = 0x80
	whiteSpaceCategory   = 0x82
	digitCategory        = 0x83
	nameStartCategory    = 0x84
	nonPrintableCategory = 0x85
)

func charCodeCategory(code int) int {
	switch {
	case code >= 0x80:
		return nameStartCategory
	case IsWhiteSpace(code):
		return whiteSpaceCategory
	case IsDigit(code):
		return digitCategory
	case IsNameStart(code):
		return nameStartCategory
	case IsNonPrintable(code):
		return nonPrintableCategory
	case code == 0:
		return eofCategory
	}
	return code
}

// IsDigit reports whether code is an ASCII digit.
func IsDigit(code int) bool {
	return code >= '0' && code <= '9'
}

// IsHexDigit reports whether code is an ASCII hex digit.
func IsHexDigit(code int) bool {
	return IsDigit(code) || (code >= 'A' && code <= 'F') || (code >= 'a' && code <= 'f')
}

// IsUppercaseLetter reports whether code is an ASCII uppercase letter.
func IsUppercaseLetter(code int) bool {
	return code >= 'A' && code <= 'Z'
}

// IsLowercaseLetter reports whether code is an ASCII lowercase letter.
func IsLowercaseLetter(code int) bool {
	return code >= 'a' && code <= 'z'
}

// IsLetter reports whether code is an ASCII letter.
func IsLetter(code int) bool {
	return IsUppercaseLetter(code) || IsLowercaseLetter(code)
}

// IsNonASCII reports whether code is U+0080 or above.
func IsNonASCII(code int) bool {
	return code >= 0x80
}

// IsNameStart reports whether code is a name-start code point.
func IsNameStart(code int) bool {
	return IsLetter(code) || IsNonASCII(code) || code == '_'
}

// IsName reports whether code is a name code point.
func IsName(code int) bool {
	return IsNameStart(code) || IsDigit(code) || code == '-'
}

// IsNonPrintable reports whether code is a non-printable code point.
func IsNonPrintable(code int) bool {
	return (code >= 0 && code <= 8) || code == 0x0B || (code >= 0x0E && code <= 0x1F) || code == 0x7F
}

// IsNewline reports whether code is LF, CR or FF.
func IsNewline(code int) bool {
	return code == '\n' || code == '\r' || code == '\f'
}

// IsWhiteSpace reports whether code is a newline, space or tab.
func IsWhiteSpace(code int) bool {
	return IsNewline(code) || code == ' ' || code == '\t'
}

// IsValidEscape reports whether the two code points start a valid escape.
// A zero second code point stands for EOF.
func IsValidEscape(first, second int) bool {
	if first != '\\' {
		return false
	}
	return !IsNewline(second) && second != 0
}

// IsIdentifierStart reports whether three code points would start an
// identifier.
func IsIdentifierStart(first, second, third int) bool {
	if first == '-' {
		return IsNameStart(second) || second == '-' || IsValidEscape(second, third)
	}
	if IsNameStart(first) {
		return true
	}
	if first == '\\' {
		return IsValidEscape(first, second)
	}
	return false
}

// IsNumberStart reports how many of the three code points start a number:
// 0 when they do not start one, otherwise the length of the sign/point/digit
// prefix (css-tree's isNumberStart).
func IsNumberStart(first, second, third int) int {
	if first == '+' || first == '-' {
		if IsDigit(second) {
			return 2
		}
		if second == '.' && IsDigit(third) {
			return 3
		}
		return 0
	}
	if first == '.' {
		if IsDigit(second) {
			return 2
		}
		return 0
	}
	if IsDigit(first) {
		return 1
	}
	return 0
}

// isBOM returns 1 when code is a byte order mark, 0 otherwise.
func isBOM(code int) int {
	if code == 0xFEFF || code == 0xFFFE {
		return 1
	}
	return 0
}

func getNewlineLength(src []uint16, offset, code int) int {
	if code == '\r' && unitAt(src, offset+1) == '\n' {
		return 2
	}
	return 1
}

// cmpChar compares the code unit at offset with a lowercase reference code,
// ASCII case-insensitively.
func cmpChar(src []uint16, offset, reference int) bool {
	if offset < 0 || offset >= len(src) {
		return false
	}
	code := int(src[offset])
	if IsUppercaseLetter(code) {
		code |= 32
	}
	return code == reference
}

// cmpStr compares src[start:end] with a lowercase reference string, ASCII
// case-insensitively.
func cmpStr(src []uint16, start, end int, reference string) bool {
	if end-start != len(reference) {
		return false
	}
	if start < 0 || end > len(src) {
		return false
	}
	for i := start; i < end; i++ {
		code := int(src[i])
		if IsUppercaseLetter(code) {
			code |= 32
		}
		if code != int(reference[i-start]) {
			return false
		}
	}
	return true
}

func findWhiteSpaceStart(src []uint16, offset int) int {
	for ; offset >= 0; offset-- {
		if !IsWhiteSpace(unitAt(src, offset)) {
			break
		}
	}
	return offset + 1
}

func findWhiteSpaceEnd(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		if !IsWhiteSpace(int(src[offset])) {
			break
		}
	}
	return offset
}

func findDecimalNumberEnd(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		if !IsDigit(int(src[offset])) {
			break
		}
	}
	return offset
}

// consumeEscaped returns the offset after the escape starting at offset
// (which points at the reverse solidus).
func consumeEscaped(src []uint16, offset int) int {
	offset += 2
	if IsHexDigit(unitAt(src, offset-1)) {
		maxOffset := min(len(src), offset+5)
		for ; offset < maxOffset; offset++ {
			if !IsHexDigit(unitAt(src, offset)) {
				break
			}
		}
		code := unitAt(src, offset)
		if IsWhiteSpace(code) {
			offset += getNewlineLength(src, offset, code)
		}
	}
	return offset
}

func consumeName(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		code := int(src[offset])
		if IsName(code) {
			continue
		}
		if IsValidEscape(code, unitAt(src, offset+1)) {
			offset = consumeEscaped(src, offset) - 1
			continue
		}
		break
	}
	return offset
}

// consumeNumber returns the end offset of the number starting at offset.
func consumeNumber(src []uint16, offset int) int {
	code := unitAt(src, offset)
	if code == '+' || code == '-' {
		offset++
		code = unitAt(src, offset)
	}
	if IsDigit(code) {
		offset = findDecimalNumberEnd(src, offset+1)
		code = unitAt(src, offset)
	}
	if code == '.' && IsDigit(unitAt(src, offset+1)) {
		offset = findDecimalNumberEnd(src, offset+2)
	}
	if cmpChar(src, offset, 'e') {
		sign := 0
		code = unitAt(src, offset+1)
		if code == '-' || code == '+' {
			sign = 1
			code = unitAt(src, offset+2)
		}
		if IsDigit(code) {
			offset = findDecimalNumberEnd(src, offset+1+sign+1)
		}
	}
	return offset
}

// consumeBadURLRemnants skips to the end of a bad URL. As in css-tree, the
// code unit following an escape is skipped as well.
func consumeBadURLRemnants(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		code := int(src[offset])
		if code == ')' {
			offset++
			break
		}
		if IsValidEscape(code, unitAt(src, offset+1)) {
			offset = consumeEscaped(src, offset)
		}
	}
	return offset
}

// decodeEscaped decodes the escape body (the text after the reverse
// solidus) into the code point it denotes.
func decodeEscaped(escaped []uint16) string {
	if len(escaped) == 1 && !IsHexDigit(int(escaped[0])) {
		return fromUnits(escaped)
	}
	code := 0
	for _, u := range escaped {
		if !IsHexDigit(int(u)) {
			break
		}
		code = code*16 + hexValue(int(u))
		if code > 0x10FFFF {
			code = 0x110000
		}
	}
	if code == 0 || (code >= 0xD800 && code <= 0xDFFF) || code > 0x10FFFF {
		code = 0xFFFD
	}
	return string(rune(code))
}

func hexValue(code int) int {
	switch {
	case code >= '0' && code <= '9':
		return code - '0'
	case code >= 'a' && code <= 'f':
		return code - 'a' + 10
	default:
		return code - 'A' + 10
	}
}

// tokenizeUnits is css-tree's tokenize(): it calls onToken for every token
// of src with its type and [start, end) offsets.
func tokenizeUnits(src []uint16, onToken func(t TokenType, start, end int)) {
	n := len(src)
	at := func(offset int) int { return unitAt(src, offset) }
	start := isBOM(at(0))
	offset := start
	var t TokenType

	// § 4.3.3. Consume a numeric token
	consumeNumericToken := func() {
		offset = consumeNumber(src, offset)
		if IsIdentifierStart(at(offset), at(offset+1), at(offset+2)) {
			t = DimensionToken
			offset = consumeName(src, offset)
			return
		}
		if at(offset) == '%' {
			t = PercentageToken
			offset++
			return
		}
		t = NumberToken
	}

	// § 4.3.6. Consume a url token
	consumeURLToken := func() {
		t = URLToken
		offset = findWhiteSpaceEnd(src, offset)
		for ; offset < n; offset++ {
			code := int(src[offset])
			switch charCodeCategory(code) {
			case ')':
				offset++
				return
			case whiteSpaceCategory:
				offset = findWhiteSpaceEnd(src, offset)
				if at(offset) == ')' || offset >= n {
					if offset < n {
						offset++
					}
					return
				}
				offset = consumeBadURLRemnants(src, offset)
				t = BadURLToken
				return
			case '"', '\'', '(', nonPrintableCategory:
				offset = consumeBadURLRemnants(src, offset)
				t = BadURLToken
				return
			case '\\':
				if IsValidEscape(code, at(offset+1)) {
					offset = consumeEscaped(src, offset) - 1
					break
				}
				offset = consumeBadURLRemnants(src, offset)
				t = BadURLToken
				return
			}
		}
	}

	// § 4.3.4. Consume an ident-like token
	consumeIdentLikeToken := func() {
		nameStart := offset
		offset = consumeName(src, offset)
		if cmpStr(src, nameStart, offset, "url") && at(offset) == '(' {
			offset = findWhiteSpaceEnd(src, offset+1)
			if at(offset) == '"' || at(offset) == '\'' {
				t = FunctionToken
				offset = nameStart + 4
				return
			}
			consumeURLToken()
			return
		}
		if at(offset) == '(' {
			t = FunctionToken
			offset++
			return
		}
		t = IdentToken
	}

	// § 4.3.5. Consume a string token
	consumeStringToken := func() {
		ending := at(offset)
		offset++
		t = StringToken
		for ; offset < n; offset++ {
			code := int(src[offset])
			switch category := charCodeCategory(code); {
			case category == ending:
				offset++
				return
			case category == whiteSpaceCategory:
				if IsNewline(code) {
					offset += getNewlineLength(src, offset, code)
					t = BadStringToken
					return
				}
			case category == '\\':
				if offset == n-1 {
					break
				}
				next := at(offset + 1)
				if IsNewline(next) {
					offset += getNewlineLength(src, offset+1, next)
				} else if IsValidEscape(code, next) {
					offset = consumeEscaped(src, offset) - 1
				}
			}
		}
	}

	// § 4.3.1. Consume a token
	for offset < n {
		code := int(src[offset])
		switch charCodeCategory(code) {
		case whiteSpaceCategory:
			t = WhiteSpaceToken
			offset = findWhiteSpaceEnd(src, offset+1)
		case '"', '\'':
			consumeStringToken()
		case '#':
			if IsName(at(offset+1)) || IsValidEscape(at(offset+1), at(offset+2)) {
				t = HashToken
				offset = consumeName(src, offset+1)
			} else {
				t = DelimToken
				offset++
			}
		case '(':
			t = LeftParenthesisToken
			offset++
		case ')':
			t = RightParenthesisToken
			offset++
		case '+':
			if IsNumberStart(code, at(offset+1), at(offset+2)) != 0 {
				consumeNumericToken()
			} else {
				t = DelimToken
				offset++
			}
		case ',':
			t = CommaToken
			offset++
		case '-':
			switch {
			case IsNumberStart(code, at(offset+1), at(offset+2)) != 0:
				consumeNumericToken()
			case at(offset+1) == '-' && at(offset+2) == '>':
				t = CDCToken
				offset += 3
			case IsIdentifierStart(code, at(offset+1), at(offset+2)):
				consumeIdentLikeToken()
			default:
				t = DelimToken
				offset++
			}
		case '.':
			if IsNumberStart(code, at(offset+1), at(offset+2)) != 0 {
				consumeNumericToken()
			} else {
				t = DelimToken
				offset++
			}
		case '/':
			if at(offset+1) == '*' {
				t = CommentToken
				offset = indexOfCommentEnd(src, offset+2)
			} else {
				t = DelimToken
				offset++
			}
		case ':':
			t = ColonToken
			offset++
		case ';':
			t = SemicolonToken
			offset++
		case '<':
			if at(offset+1) == '!' && at(offset+2) == '-' && at(offset+3) == '-' {
				t = CDOToken
				offset += 4
			} else {
				t = DelimToken
				offset++
			}
		case '@':
			if IsIdentifierStart(at(offset+1), at(offset+2), at(offset+3)) {
				t = AtKeywordToken
				offset = consumeName(src, offset+1)
			} else {
				t = DelimToken
				offset++
			}
		case '[':
			t = LeftSquareBracketToken
			offset++
		case '\\':
			if IsValidEscape(code, at(offset+1)) {
				consumeIdentLikeToken()
			} else {
				t = DelimToken
				offset++
			}
		case ']':
			t = RightSquareBracketToken
			offset++
		case '{':
			t = LeftCurlyBracketToken
			offset++
		case '}':
			t = RightCurlyBracketToken
			offset++
		case digitCategory:
			consumeNumericToken()
		case nameStartCategory:
			consumeIdentLikeToken()
		default:
			t = DelimToken
			offset++
		}

		onToken(t, start, offset)
		start = offset
	}
}

// indexOfCommentEnd returns the offset after the first "*/" at or after
// from, or len(src) for an unterminated comment.
func indexOfCommentEnd(src []uint16, from int) int {
	for i := from; i+1 < len(src); i++ {
		if src[i] == '*' && src[i+1] == '/' {
			return i + 2
		}
	}
	return len(src)
}
