package clean

import (
	"testing"

	"github.com/css/csso-go/internal/csso/usage"
	"github.com/css/csso-go/internal/csstree"
)

// The expectations below were recorded from csso 5.0.5 on Node.js 24 by
// running csso's clean pass alone on the parsed stylesheet.

func TestRun(t *testing.T) {
	tests := []struct {
		css   string
		usage *usage.Index
		want  string
	}{
		{`/* x */a { color : red ; ; } b{} @media print{} @media {a{x:1}} @supports (x:y){}`, nil, `a{color:red}`},
		{`a{x:1}@charset "a";@import "b";`, nil, `a{x:1}`},
		{`@charset "a";@import "b";@font-face{src:x}@import "c";`, nil, `@charset "a";@import "b";@font-face{src:x}`},
		{`@media print{a{x:y}}@import "b";`, nil, `@media print{a{x:y}}`},
		{`*.a, *#b, *[c], *:d, *::e, * f, *|*.g { x:y }`, nil, `.a,#b,[c],:d,::e,* f,*|*.g{x:y}`},
		{`a{--x:  y  ;--z:   ;}`, nil, `a{--x:y;--z:   }`},
		{
			`a,.b,#c,div,.d .e{x:y}`,
			&usage.Index{Whitelist: usage.NewList([]string{"A"}, nil, []string{"b"})},
			`a,.b,#c{x:y}`,
		},
		{
			`.a:not(.b),:is(.c,.d){x:y}`,
			&usage.Index{Whitelist: usage.NewList(nil, nil, []string{"a", "c"})},
			`.a:not(.b),:is(.c){x:y}`,
		},
		{
			`@keyframes k{from{x:1}}.a{x:1}`,
			&usage.Index{Whitelist: usage.NewList(nil, nil, []string{})},
			`@keyframes k{from{x:1}}`,
		},
		{
			`.a,.b{x:y}`,
			&usage.Index{Blacklist: usage.NewList(nil, nil, []string{"b"})},
			`.a{x:y}`,
		},
	}
	for _, tt := range tests {
		node, err := csstree.Parse(tt.css, nil)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.css, err)
		}
		ast := node.(*csstree.StyleSheet)
		ast.FirstAtrulesAllowed = true
		if err := Run(ast, Options{Usage: tt.usage}); err != nil {
			t.Fatalf("Run(%q): %v", tt.css, err)
		}
		if got := csstree.Generate(ast); got != tt.want {
			t.Errorf("Run(%q) = %q; want %q", tt.css, got, tt.want)
		}
	}
}

func TestRunNodeWithoutList(t *testing.T) {
	// csso fails to remove an empty at-rule that is not in a list
	node, err := csstree.Parse("@media print{}", &csstree.ParseOptions{Context: csstree.ContextAtrule})
	if err != nil {
		t.Fatal(err)
	}
	stylesheet := &csstree.StyleSheet{Children: csstree.NewList(), FirstAtrulesAllowed: true}
	err = RunNode(node, stylesheet, Options{})
	if err == nil || err.Error() != "TypeError: Cannot read properties of undefined (reading 'remove')" {
		t.Errorf("RunNode error = %v", err)
	}
}
