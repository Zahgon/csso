// Package jscompat reproduces the pieces of JavaScript runtime behaviour that
// csso's output depends on: thrown error values, string ordering by UTF-16
// code units, the ECMAScript white space set and Number#toString.
package jscompat

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Error is a JavaScript exception as csso would throw it: Name is the
// constructor name ("Error", "TypeError", "SyntaxError") and Message the
// error message.
type Error struct {
	Name    string
	Message string
}

// Error returns the error as JavaScript's String(error) renders it.
func (e *Error) Error() string {
	return e.Name + ": " + e.Message
}

// NewError returns a plain JavaScript Error.
func NewError(message string) *Error {
	return &Error{Name: "Error", Message: message}
}

// NewTypeError returns a JavaScript TypeError.
func NewTypeError(message string) *Error {
	return &Error{Name: "TypeError", Message: message}
}

// ReadPropertyError is the TypeError V8 throws when reading property of
// base, where base is "undefined" or "null".
func ReadPropertyError(base, property string) *Error {
	return NewTypeError("Cannot read properties of " + base + " (reading '" + property + "')")
}

// Compare orders a and b the way JavaScript's relational operators do:
// lexicographically by UTF-16 code units. It returns -1, 0 or +1.
func Compare(a, b string) int {
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb {
			ha, la := utf16Units(ra)
			hb, lb := utf16Units(rb)
			if ha != hb {
				return cmp(ha, hb)
			}
			return cmp(la, lb)
		}
		a, b = a[na:], b[nb:]
	}
	return cmp(len(a), len(b))
}

// utf16Units returns the UTF-16 encoding of r as a (high, low) pair; low is
// -1 for a code point in the Basic Multilingual Plane.
func utf16Units(r rune) (int, int) {
	if r < 0x10000 {
		return int(r), -1
	}
	r -= 0x10000
	return 0xD800 + int(r>>10), 0xDC00 + int(r&0x3FF)
}

func cmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// IsSpace reports whether r matches JavaScript's \s, i.e. is a WhiteSpace or
// LineTerminator code point.
func IsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00A0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// TrimSpace is String#trim.
func TrimSpace(s string) string {
	return strings.TrimFunc(s, IsSpace)
}

// HasNonSpace reports whether s contains a character other than white space
// (JavaScript's /\S/.test(s)).
func HasNonSpace(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !IsSpace(r) }) != -1
}

// NumberToString formats f as JavaScript's Number#toString() (radix 10):
// the shortest round-tripping digits, in exponential notation below 1e-6 and
// from 1e21 on.
func NumberToString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	case f < 0:
		return "-" + NumberToString(-f)
	}

	// "d.ddde±x": the digits and the decimal exponent of the shortest form
	mantissa, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	k, n := len(digits), e+1

	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	}

	sign := "+"
	if n-1 < 0 {
		sign = "-"
	}
	exponent := strconv.Itoa(max(n-1, 1-n))
	if k == 1 {
		return digits + "e" + sign + exponent
	}
	return digits[:1] + "." + digits[1:] + "e" + sign + exponent
}
