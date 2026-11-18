package lexer

import (
	"errors"
	"strconv"

	"github.com/css/csso-go/internal/csstree"
)

// toLowerCase is JavaScript's String#toLowerCase, which css-tree applies to
// names before comparing them.
func toLowerCase(s string) string {
	return csstree.LowerCase(s)
}

// charAt returns the byte at index i of s, or 0 when i is out of range
// (css-tree's safe charCodeAt). Non-ASCII characters yield a byte >= 0x80,
// which every css-tree check treats like the UTF-16 code unit it would see.
func charAt(s string, i int) int {
	if i >= 0 && i < len(s) {
		return int(s[i])
	}
	return 0
}

// equalsLower reports whether s equals the lower-case ASCII reference,
// folding only A-Z as css-tree's cmpStr and areStringsEqualCaseInsensitive
// do.
func equalsLower(s, reference string) bool {
	if len(s) != len(reference) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c |= 0x20
		}
		if c != reference[i] {
			return false
		}
	}
	return true
}

func equalsLowerAny(s string, references []string) bool {
	for _, reference := range references {
		if equalsLower(s, reference) {
			return true
		}
	}
	return false
}

func decimalEnd(s string, i int) int {
	for i < len(s) && csstree.IsDigit(int(s[i])) {
		i++
	}
	return i
}

// numberEnd returns the end of the number at the start of s (css-tree's
// consumeNumber(s, 0)).
func numberEnd(s string) int {
	i := 0
	if c := charAt(s, i); c == '+' || c == '-' {
		i++
	}
	if csstree.IsDigit(charAt(s, i)) {
		i = decimalEnd(s, i+1)
	}
	if charAt(s, i) == '.' && csstree.IsDigit(charAt(s, i+1)) {
		i = decimalEnd(s, i+2)
	}
	if c := charAt(s, i); c == 'e' || c == 'E' {
		sign := 0
		c = charAt(s, i+1)
		if c == '-' || c == '+' {
			sign = 1
			c = charAt(s, i+2)
		}
		if csstree.IsDigit(c) {
			i = decimalEnd(s, i+1+sign+1)
		}
	}
	return i
}

// numberValue converts a numeric prefix the way JavaScript's Number() does:
// the empty string is 0 and an unparsable string is NaN (reported as !ok).
func numberValue(s string) (v float64, ok bool) {
	if s == "" {
		return 0, true
	}
	// Number() rejects the forms ParseFloat accepts beyond CSS numbers
	// (hex floats, underscores, "inf"); numberEnd guards against them.
	if numberEnd(s) != len(s) {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// out-of-range literals saturate to ±Inf, as in JavaScript
		if errors.Is(err, strconv.ErrRange) {
			return v, true
		}
		return 0, false
	}
	return v, true
}
