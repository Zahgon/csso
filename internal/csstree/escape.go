package csstree

import (
	"strconv"
	"strings"
)

// This file ports css-tree's lib/utils/string.js, url.js and ident.js.

// decodeEscapes decodes src[start:end+1], resolving escapes. When the text
// ends with a lone reverse solidus that is not the last code unit of src
// (an escaped closing quote or parenthesis), css-tree returns only the text
// after it; that quirk is kept.
func decodeEscapes(src []uint16, start, end int) string {
	var decoded []uint16
	for i := start; i <= end; i++ {
		code := src[i]
		if code != '\\' {
			decoded = append(decoded, code)
			continue
		}

		if i == end {
			// at EOF do nothing, otherwise keep the escaped terminator
			if i != len(src)-1 {
				decoded = append([]uint16(nil), src[i+1:]...)
			}
			break
		}

		i++
		next := int(src[i])
		if IsValidEscape('\\', next) {
			escapeStart := i - 1
			escapeEnd := consumeEscaped(src, escapeStart)
			i = escapeEnd - 1
			decoded = append(decoded, toUnits(decodeEscaped(src[escapeStart+1:escapeEnd]))...)
		} else if next == '\r' && unitAt(src, i+1) == '\n' {
			// an escaped CR LF is a line continuation
			i++
		}
	}
	return fromUnits(decoded)
}

// DecodeString decodes a CSS string token (quoted or not) into its value.
func DecodeString(str string) string {
	src := toUnits(str)
	n := len(src)
	if n == 0 {
		return ""
	}
	first := src[0]
	start := 0
	if first == '"' || first == '\'' {
		start = 1
	}
	end := n - 1
	if start == 1 && n > 1 && src[n-1] == first {
		end = n - 2
	}
	return decodeEscapes(src, start, end)
}

// EncodeString serializes str as a CSS string in double quotes, or in
// single quotes when apostrophe is set
// (https://drafts.csswg.org/cssom/#serialize-a-string).
func EncodeString(str string, apostrophe bool) string {
	quote := byte('"')
	if apostrophe {
		quote = '\''
	}

	var b strings.Builder
	b.WriteByte(quote)
	wsBeforeHexIsNeeded := false
	for _, r := range str {
		code := int(r)
		switch {
		case code == 0:
			b.WriteRune('�')
		case code <= 0x1F || code == 0x7F:
			b.WriteByte('\\')
			b.WriteString(strconv.FormatInt(int64(code), 16))
			wsBeforeHexIsNeeded = true
		case code == int(quote) || code == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
			wsBeforeHexIsNeeded = false
		default:
			if wsBeforeHexIsNeeded && (IsHexDigit(code) || IsWhiteSpace(code)) {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			wsBeforeHexIsNeeded = false
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// DecodeURL decodes a <url-token> (including the "url(" prefix) into the
// URL it denotes.
func DecodeURL(str string) string {
	src := toUnits(str)
	n := len(src)
	start := 4 // len("url(")
	end := n - 1
	if n > 0 && src[n-1] == ')' {
		end = n - 2
	}
	for start < end && IsWhiteSpace(unitAt(src, start)) {
		start++
	}
	for start < end && IsWhiteSpace(unitAt(src, end)) {
		end--
	}
	if start > end {
		return ""
	}
	return decodeEscapes(src, start, end)
}

// EncodeURL serializes str as an unquoted url() token.
func EncodeURL(str string) string {
	var b strings.Builder
	b.WriteString("url(")
	wsBeforeHexIsNeeded := false
	for _, r := range str {
		code := int(r)
		switch {
		case code == 0:
			b.WriteRune('�')
		case code <= 0x1F || code == 0x7F:
			b.WriteByte('\\')
			b.WriteString(strconv.FormatInt(int64(code), 16))
			wsBeforeHexIsNeeded = true
		case code == ' ' || code == '\\' || code == '"' || code == '\'' || code == '(' || code == ')':
			b.WriteByte('\\')
			b.WriteRune(r)
			wsBeforeHexIsNeeded = false
		default:
			if wsBeforeHexIsNeeded && IsHexDigit(code) {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			wsBeforeHexIsNeeded = false
		}
	}
	b.WriteByte(')')
	return b.String()
}

// DecodeIdent decodes the escapes of an identifier; a trailing reverse
// solidus (an escape at EOF) is dropped.
func DecodeIdent(str string) string {
	src := toUnits(str)
	return decodeEscapes(src, 0, len(src)-1)
}

// EncodeIdent serializes str as a CSS identifier
// (https://drafts.csswg.org/cssom/#serialize-an-identifier).
func EncodeIdent(str string) string {
	if str == "-" {
		return "\\-"
	}

	var b strings.Builder
	first := firstUnit(str)
	i := 0
	for _, r := range str {
		code := int(r)
		switch {
		case code == 0:
			b.WriteRune('�')
		case code <= 0x1F || code == 0x7F ||
			(IsDigit(code) && (i == 0 || (i == 1 && first == '-'))):
			b.WriteByte('\\')
			b.WriteString(strconv.FormatInt(int64(code), 16))
			b.WriteByte(' ')
		case IsName(code):
			b.WriteRune(r)
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
		if r >= 0x10000 {
			i += 2
		} else {
			i++
		}
	}
	return b.String()
}
