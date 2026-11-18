package csstree

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// css-tree works on JavaScript strings, so every offset it reports (token
// boundaries, node locations, source map columns) counts UTF-16 code units.
// The helpers below convert between Go strings and UTF-16 code unit slices so
// the port can reproduce those offsets exactly.

// toUnits converts s to UTF-16 code units.
func toUnits(s string) []uint16 {
	if isASCII(s) {
		units := make([]uint16, len(s))
		for i := 0; i < len(s); i++ {
			units[i] = uint16(s[i])
		}
		return units
	}
	return utf16.Encode([]rune(s))
}

// fromUnits converts UTF-16 code units back to a Go string. Unpaired
// surrogates become U+FFFD.
func fromUnits(units []uint16) string {
	ascii := true
	for _, u := range units {
		if u >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		b := make([]byte, len(units))
		for i, u := range units {
			b[i] = byte(u)
		}
		return string(b)
	}
	return string(utf16.Decode(units))
}

// sliceUnits returns units[start:end] as a string with String#slice's
// clamping of out-of-range bounds.
func sliceUnits(units []uint16, start, end int) string {
	start = min(start, len(units))
	end = min(end, len(units))
	if start >= end {
		return ""
	}
	return fromUnits(units[start:end])
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// UTF16Len returns the length of s in UTF-16 code units (JavaScript's
// String#length).
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// firstUnit returns the first UTF-16 code unit of s (String#charCodeAt(0)),
// or -1 for an empty string (JavaScript's NaN never equals a character).
func firstUnit(s string) int {
	if s == "" {
		return -1
	}
	if s[0] < utf8.RuneSelf {
		return int(s[0])
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r >= 0x10000 {
		hi, _ := utf16.EncodeRune(r)
		return int(hi)
	}
	return int(r)
}

// unitAt returns units[i], or 0 when i is out of range (the tokenizer's
// getCharCode convention).
func unitAt(units []uint16, i int) int {
	if i >= 0 && i < len(units) {
		return int(units[i])
	}
	return 0
}

// LowerCase reproduces JavaScript's String#toLowerCase, including the
// context-free special casing of U+0130 (to "i\u0307") and the Final_Sigma
// rule.
func LowerCase(s string) string {
	if isASCII(s) {
		return strings.ToLower(s)
	}
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		switch {
		case r == 'İ':
			b.WriteString("i̇")
		case r == 'Σ' && isFinalSigma(runes, i):
			b.WriteRune('ς')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// isFinalSigma implements the Unicode Final_Sigma casing context: the sigma
// is preceded by a cased letter and not followed by one, ignoring
// case-ignorable characters in both directions.
func isFinalSigma(runes []rune, i int) bool {
	before := false
	for j := i - 1; j >= 0; j-- {
		if isCaseIgnorable(runes[j]) {
			continue
		}
		before = isCased(runes[j])
		break
	}
	if !before {
		return false
	}
	for j := i + 1; j < len(runes); j++ {
		if isCaseIgnorable(runes[j]) {
			continue
		}
		return !isCased(runes[j])
	}
	return true
}

func isCased(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) ||
		unicode.Is(unicode.Other_Lowercase, r) || unicode.Is(unicode.Other_Uppercase, r)
}

func isCaseIgnorable(r rune) bool {
	switch r {
	case '\'', '.', ':', '^', '`', 0x00B7, 0x2018, 0x2019, 0x2024, 0x2027:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
