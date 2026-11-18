package lexer

import (
	"strings"

	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// genericFunc matches a generic type at tok, the current token (nil at the
// end of the input). next(offset) returns the token offset positions after
// tok, whitespace included, or nil past the end. opts is the range
// restriction in effect. The result is the number of tokens consumed; 0 means
// no match.
type genericFunc func(tok *token, next func(offset int) *token, opts *defsyntax.Range) int

// cssWideKeywords are the CSS-wide keywords of css-cascade-5, which every
// property accepts (css-tree's generic-const.js).
var cssWideKeywords = []string{"initial", "inherit", "unset", "revert", "revert-layer"}

var calcFunctionNames = []string{"calc(", "-moz-calc(", "-webkit-calc("}

// balancePair maps an opening token type to the type that closes it.
var balancePair = map[csstree.TokenType]csstree.TokenType{
	csstree.FunctionToken:          csstree.RightParenthesisToken,
	csstree.LeftParenthesisToken:   csstree.RightParenthesisToken,
	csstree.LeftSquareBracketToken: csstree.RightSquareBracketToken,
	csstree.LeftCurlyBracketToken:  csstree.RightCurlyBracketToken,
}

// Units of the dimension types, as listed by css-tree 2.2.0.
var (
	lengthUnits = []string{
		// absolute length units https://www.w3.org/TR/css-values-3/#lengths
		"cm", "mm", "q", "in", "pt", "pc", "px",
		// font-relative length units https://drafts.csswg.org/css-values-4/#font-relative-lengths
		"em", "rem",
		"ex", "rex",
		"cap", "rcap",
		"ch", "rch",
		"ic", "ric",
		"lh", "rlh",
		// viewport-percentage lengths https://drafts.csswg.org/css-values-4/#viewport-relative-lengths
		"vw", "svw", "lvw", "dvw",
		"vh", "svh", "lvh", "dvh",
		"vi", "svi", "lvi", "dvi",
		"vb", "svb", "lvb", "dvb",
		"vmin", "svmin", "lvmin", "dvmin",
		"vmax", "svmax", "lvmax", "dvmax",
		// container relative lengths https://drafts.csswg.org/css-contain-3/#container-lengths
		"cqw", "cqh", "cqi", "cqb", "cqmin", "cqmax",
	}
	angleUnits      = []string{"deg", "grad", "rad", "turn"} // https://www.w3.org/TR/css-values-3/#angles
	timeUnits       = []string{"s", "ms"}                    // https://www.w3.org/TR/css-values-3/#time
	frequencyUnits  = []string{"hz", "khz"}                  // https://www.w3.org/TR/css-values-3/#frequency
	resolutionUnits = []string{"dpi", "dpcm", "dppx", "x"}   // https://www.w3.org/TR/css-values-3/#resolution
	flexUnits       = []string{"fr"}                         // https://drafts.csswg.org/css-grid/#fr-unit
	decibelUnits    = []string{"db"}                         // https://www.w3.org/TR/css3-speech/#mixing-props-voice-volume
	semitonesUnits  = []string{"st"}                         // https://www.w3.org/TR/css3-speech/#voice-props-voice-pitch
)

// genericTypes are the types css-tree implements in code rather than with a
// definition syntax; they replace mdn-data definitions of the same name.
var genericTypes = map[string]genericFunc{
	// token types
	"ident-token":          tokenType(csstree.IdentToken),
	"function-token":       tokenType(csstree.FunctionToken),
	"at-keyword-token":     tokenType(csstree.AtKeywordToken),
	"hash-token":           tokenType(csstree.HashToken),
	"string-token":         tokenType(csstree.StringToken),
	"bad-string-token":     tokenType(csstree.BadStringToken),
	"url-token":            tokenType(csstree.URLToken),
	"bad-url-token":        tokenType(csstree.BadURLToken),
	"delim-token":          tokenType(csstree.DelimToken),
	"number-token":         tokenType(csstree.NumberToken),
	"percentage-token":     tokenType(csstree.PercentageToken),
	"dimension-token":      tokenType(csstree.DimensionToken),
	"whitespace-token":     tokenType(csstree.WhiteSpaceToken),
	"CDO-token":            tokenType(csstree.CDOToken),
	"CDC-token":            tokenType(csstree.CDCToken),
	"colon-token":          tokenType(csstree.ColonToken),
	"semicolon-token":      tokenType(csstree.SemicolonToken),
	"comma-token":          tokenType(csstree.CommaToken),
	"[-token":              tokenType(csstree.LeftSquareBracketToken),
	"]-token":              tokenType(csstree.RightSquareBracketToken),
	"(-token":              tokenType(csstree.LeftParenthesisToken),
	")-token":              tokenType(csstree.RightParenthesisToken),
	"{-token":              tokenType(csstree.LeftCurlyBracketToken),
	"}-token":              tokenType(csstree.RightCurlyBracketToken),
	"string":               tokenType(csstree.StringToken),
	"ident":                tokenType(csstree.IdentToken),
	"custom-ident":         customIdent,
	"custom-property-name": customPropertyName,
	"hex-color":            hexColor,
	"id-selector":          idSelector,
	"an-plus-b":            anPlusB,
	"urange":               urange,
	"declaration-value":    declarationValue,
	"any-value":            anyValue,
	"dimension":            calc(dimension(nil)),
	"angle":                calc(dimension(angleUnits)),
	"decibel":              calc(dimension(decibelUnits)),
	"frequency":            calc(dimension(frequencyUnits)),
	"flex":                 calc(dimension(flexUnits)),
	"length":               calc(zero(dimension(lengthUnits))),
	"resolution":           calc(dimension(resolutionUnits)),
	"semitones":            calc(dimension(semitonesUnits)),
	"time":                 calc(dimension(timeUnits)),
	"percentage":           calc(percentage),
	"zero":                 zero(nil),
	"number":               calc(number),
	"integer":              calc(integer),
}

// isPostfixIEHack reports whether s ends at offset with an IE postfix hack,
// i.e. `\0` or `\9` as in `123\0` or `123px\9`.
func isPostfixIEHack(s string, offset int) bool {
	return offset == len(s)-2 && charAt(s, offset) == '\\' && csstree.IsDigit(charAt(s, offset+1))
}

// outOfRange checks the numeric prefix value[:numEnd] against a range
// restriction. Like css-tree it skips bounds that carry a unit, which would
// need a unit conversion.
func outOfRange(opts *defsyntax.Range, value string, numEnd int) bool {
	if opts == nil {
		return false
	}
	if numEnd != len(value) {
		value = value[:numEnd]
	}
	num, ok := numberValue(value)
	if !ok {
		return true
	}
	if opts.Min != nil && opts.Min.Unit == "" && num < opts.Min.Value {
		return true
	}
	if opts.Max != nil && opts.Max.Unit == "" && num > opts.Max.Value {
		return true
	}
	return false
}

// consumeFunction returns the number of tokens up to and including the
// parenthesis that closes the function token tok.
func consumeFunction(tok *token, next func(int) *token) int {
	var closeType csstree.TokenType
	var stash []csstree.TokenType
	length := 0

scan:
	for ; tok != nil; tok = next(length) {
		switch tok.typ {
		case csstree.RightCurlyBracketToken, csstree.RightParenthesisToken, csstree.RightSquareBracketToken:
			if tok.typ != closeType {
				break scan
			}
			closeType, stash = stash[len(stash)-1], stash[:len(stash)-1]
			if len(stash) == 0 {
				length++
				break scan
			}
		case csstree.FunctionToken, csstree.LeftParenthesisToken, csstree.LeftSquareBracketToken, csstree.LeftCurlyBracketToken:
			stash = append(stash, closeType)
			closeType = balancePair[tok.typ]
		}
		length++
	}

	return length
}

// calc lets a numeric type also match calc() and its prefixed variants,
// whose content css-tree does not validate (https://drafts.csswg.org/css-values/#calc-notation).
func calc(next genericFunc) genericFunc {
	return func(tok *token, nextToken func(int) *token, opts *defsyntax.Range) int {
		if tok == nil {
			return 0
		}
		if tok.typ == csstree.FunctionToken && equalsLowerAny(tok.value, calcFunctionNames) {
			return consumeFunction(tok, nextToken)
		}
		return next(tok, nextToken, opts)
	}
}

func tokenType(expected csstree.TokenType) genericFunc {
	return func(tok *token, _ func(int) *token, _ *defsyntax.Range) int {
		if tok == nil || tok.typ != expected {
			return 0
		}
		return 1
	}
}

// customIdent matches a <custom-ident>: any identifier except the CSS-wide
// keywords and `default` (https://drafts.csswg.org/css-values-4/#custom-idents).
// Like css-tree it does not exclude the keywords of the property's own syntax.
func customIdent(tok *token, _ func(int) *token, _ *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.IdentToken {
		return 0
	}
	name := toLowerCase(tok.value)
	if equalsLowerAny(name, cssWideKeywords) || equalsLower(name, "default") {
		return 0
	}
	return 1
}

// customPropertyName matches an identifier starting with two dashes; like
// css-tree it accepts `--` itself (https://drafts.csswg.org/css-variables/#typedef-custom-property-name).
func customPropertyName(tok *token, _ func(int) *token, _ *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.IdentToken {
		return 0
	}
	if !strings.HasPrefix(tok.value, "--") {
		return 0
	}
	return 1
}

// hexColor matches a hash token of 3, 4, 6 or 8 hexadecimal digits
// (https://drafts.csswg.org/css-color-4/#hex-notation).
func hexColor(tok *token, _ func(int) *token, _ *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.HashToken {
		return 0
	}
	switch len(tok.value) {
	case 4, 5, 7, 9: // #rgb, #rgba, #rrggbb, #rrggbbaa
	default:
		return 0
	}
	for i := 1; i < len(tok.value); i++ {
		if !csstree.IsHexDigit(int(tok.value[i])) {
			return 0
		}
	}
	return 1
}

func idSelector(tok *token, _ func(int) *token, _ *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.HashToken {
		return 0
	}
	if !csstree.IsIdentifierStart(charAt(tok.value, 1), charAt(tok.value, 2), charAt(tok.value, 3)) {
		return 0
	}
	return 1
}

// declarationValue matches <declaration-value>: one or more tokens without
// bad strings or urls, unmatched closing brackets, or top-level semicolons
// and `!` delimiters (https://drafts.csswg.org/css-syntax/#any-value).
func declarationValue(tok *token, next func(int) *token, _ *defsyntax.Range) int {
	return balancedValue(tok, next, true)
}

// anyValue matches <any-value>, which is <declaration-value> that also allows
// top-level semicolons and `!` delimiters.
func anyValue(tok *token, next func(int) *token, _ *defsyntax.Range) int {
	return balancedValue(tok, next, false)
}

func balancedValue(tok *token, next func(int) *token, declaration bool) int {
	var closeType csstree.TokenType
	var stash []csstree.TokenType
	length := 0

scan:
	for ; tok != nil; tok = next(length) {
		switch tok.typ {
		case csstree.BadStringToken, csstree.BadURLToken:
			break scan

		case csstree.RightCurlyBracketToken, csstree.RightParenthesisToken, csstree.RightSquareBracketToken:
			if tok.typ != closeType {
				break scan
			}
			closeType, stash = stash[len(stash)-1], stash[:len(stash)-1]

		case csstree.SemicolonToken:
			if declaration && len(stash) == 0 {
				break scan
			}

		case csstree.DelimToken:
			if declaration && len(stash) == 0 && tok.value == "!" {
				break scan
			}

		case csstree.FunctionToken, csstree.LeftParenthesisToken, csstree.LeftSquareBracketToken, csstree.LeftCurlyBracketToken:
			stash = append(stash, closeType)
			closeType = balancePair[tok.typ]
		}
		length++
	}

	return length
}

// dimension matches a dimension token whose unit is one of units (any unit
// when units is nil), tolerating an IE postfix hack after the unit.
func dimension(units []string) genericFunc {
	return func(tok *token, _ func(int) *token, opts *defsyntax.Range) int {
		if tok == nil || tok.typ != csstree.DimensionToken {
			return 0
		}

		numEnd := numberEnd(tok.value)

		if units != nil {
			unit := tok.value[numEnd:]
			if hack := strings.IndexByte(unit, '\\'); hack != -1 && isPostfixIEHack(tok.value, numEnd+hack) {
				unit = unit[:hack]
			}
			if !equalsLowerAny(toLowerCase(unit), units) {
				return 0
			}
		}

		if outOfRange(opts, tok.value, numEnd) {
			return 0
		}
		return 1
	}
}

// percentage matches a <percentage-token> (https://drafts.csswg.org/css-values-4/#percentages).
func percentage(tok *token, _ func(int) *token, opts *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.PercentageToken {
		return 0
	}
	if outOfRange(opts, tok.value, len(tok.value)-1) {
		return 0
	}
	return 1
}

// zero matches a literal number token of value 0, falling back to next
// (https://drafts.csswg.org/css-values-4/#numbers: calc(0) is not <zero>).
func zero(next genericFunc) genericFunc {
	return func(tok *token, nextToken func(int) *token, opts *defsyntax.Range) int {
		if tok != nil && tok.typ == csstree.NumberToken {
			if v, ok := numberValue(tok.value); ok && v == 0 {
				return 1
			}
		}
		if next == nil {
			return 0
		}
		return next(tok, nextToken, opts)
	}
}

// number matches <number> (https://drafts.csswg.org/css-values-4/#numbers).
// Like css-tree it inspects only the token value, not the token type, and
// tolerates an IE postfix hack.
func number(tok *token, _ func(int) *token, opts *defsyntax.Range) int {
	if tok == nil {
		return 0
	}
	numEnd := numberEnd(tok.value)
	if numEnd != len(tok.value) && !isPostfixIEHack(tok.value, numEnd) {
		return 0
	}
	if outOfRange(opts, tok.value, numEnd) {
		return 0
	}
	return 1
}

// integer matches <integer>: a number token of optionally signed decimal
// digits (https://drafts.csswg.org/css-values-4/#integers).
func integer(tok *token, _ func(int) *token, opts *defsyntax.Range) int {
	if tok == nil || tok.typ != csstree.NumberToken {
		return 0
	}

	i := 0
	if c := charAt(tok.value, 0); c == '+' || c == '-' {
		i = 1
	}
	for ; i < len(tok.value); i++ {
		if !csstree.IsDigit(int(tok.value[i])) {
			return 0
		}
	}

	if outOfRange(opts, tok.value, i) {
		return 0
	}
	return 1
}
