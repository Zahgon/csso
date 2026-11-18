package jscompat

import (
	"math"
	"testing"
)

// The expectations below were recorded from Node.js 24.

func TestNumberToString(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{0.1, "0.1"},
		{0.5, "0.5"},
		{1.0 / 3, "0.3333333333333333"},
		{255, "255"},
		{1e21, "1e+21"},
		{1e-7, "1e-7"},
		{123456789012345680000, "123456789012345680000"},
		{0.000001, "0.000001"},
		{0.0000012345, "0.0000012345"},
		{-0.5, "-0.5"},
		{1.5e300, "1.5e+300"},
		{1 << 53, "9007199254740992"},
		{100, "100"},
		{5e-324, "5e-324"},
	}
	for _, tt := range tests {
		if got := NumberToString(tt.in); got != tt.want {
			t.Errorf("NumberToString(%v) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"a", "b", -1},
		{"b", "a", 1},
		{"a", "a", 0},
		{"a", "ab", -1},
		{"\uffff", "\U0001F600", 1}, // a surrogate pair sorts before U+FFFF
		{"\U0001F600", "\U0001F601", -1},
		{"é", "z", 1},
		{"Z", "a", -1},
	}
	for _, tt := range tests {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Errorf("Compare(%q, %q) = %d; want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		in       string
		want     string
		nonSpace bool
	}{
		{"\u00a0x\ufeff", "x", true},
		{"\u0085x", "\u0085x", true}, // NEL is not JavaScript white space
		{" \t\n", "", false},
		{"x\u2028", "x", true},
	}
	for _, tt := range tests {
		if got := TrimSpace(tt.in); got != tt.want {
			t.Errorf("TrimSpace(%q) = %q; want %q", tt.in, got, tt.want)
		}
		if got := HasNonSpace(tt.in); got != tt.nonSpace {
			t.Errorf("HasNonSpace(%q) = %v; want %v", tt.in, got, tt.nonSpace)
		}
	}
}

func TestReadPropertyError(t *testing.T) {
	err := ReadPropertyError("null", "first")
	if err.Name != "TypeError" || err.Error() != "TypeError: Cannot read properties of null (reading 'first')" {
		t.Errorf("ReadPropertyError = %#v", err)
	}
}
