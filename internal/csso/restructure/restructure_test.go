package restructure

import (
	"fmt"
	"slices"
	"testing"

	"github.com/css/csso-go/internal/csso/clean"
	"github.com/css/csso-go/internal/csso/replace"
	"github.com/css/csso-go/internal/csstree"
)

// The expectations below were recorded from csso 5.0.5 on Node.js 24 by
// running csso's clean, replace, prepare and restructure steps one by one.

// prepared parses css, numbers its blocks as csso's compress does and runs
// clean and replace.
func prepared(t *testing.T, css string) *csstree.StyleSheet {
	t.Helper()
	node, err := csstree.Parse(css, nil)
	if err != nil {
		t.Fatalf("Parse(%q): %v", css, err)
	}
	ast := node.(*csstree.StyleSheet)
	ast.FirstAtrulesAllowed = true
	ast.ID = 1
	seed := 2
	csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Atrule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
			if block := node.(*csstree.Atrule).Block; block != nil {
				block.ID = seed
				seed++
			}
			return csstree.WalkContinue
		},
	})
	if err := clean.Run(ast, clean.Options{}); err != nil {
		t.Fatalf("clean %q: %v", css, err)
	}
	if err := replace.Run(ast); err != nil {
		t.Fatalf("replace %q: %v", css, err)
	}
	return ast
}

func TestSteps(t *testing.T) {
	// want holds the CSS after prepare and after each of the seven steps
	tests := []struct {
		css  string
		want []string
	}{
		{`.a{color:red}.b{color:red}`, []string{
			`.a{color:red}.b{color:red}`,
			`.a{color:red}.b{color:red}`,
			`.a,.b{color:red}`,
			`.a{color:red}.b{color:red}`,
			`.a{color:red}.b{color:red}`,
			`.a{color:red}.b{color:red}`,
			`.a,.b{color:red}`,
			`.a,.b{color:red}`,
		}},
		{`.a,.b{color:red;margin:0}.a{padding:0}`, []string{
			`.a,.b{color:red;margin:0}.a{padding:0}`,
			`.a,.b{color:red;margin:0}.a{padding:0}`,
			`.a,.b{color:red;margin:0}.a{padding:0}`,
			`.a{color:red;margin:0}.b{color:red;margin:0}.a{padding:0}`,
			`.a{color:red;margin:0}.b{color:red;margin:0}.a{padding:0}`,
			`.a{color:red;margin:0}.b{color:red;margin:0}.a{padding:0}`,
			`.a,.b{color:red;margin:0}.a{padding:0}`,
			`.a,.b{color:red;margin:0}.a{padding:0}`,
		}},
		{`a{margin-top:1px;margin-right:1px;margin-bottom:1px;margin-left:1px}`, []string{
			`a{margin-top:1px;margin-right:1px;margin-bottom:1px;margin-left:1px}`,
			`a{margin-top:1px;margin-right:1px;margin-bottom:1px;margin-left:1px}`,
			`a{margin-top:1px;margin-right:1px;margin-bottom:1px;margin-left:1px}`,
			`a{margin-top:1px;margin-right:1px;margin-bottom:1px;margin-left:1px}`,
			`a{margin:1px}`,
			`a{margin:1px}`,
			`a{margin:1px}`,
			`a{margin:1px}`,
		}},
		{`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`, []string{
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{color:red;width:2px}.c{color:red}`,
			`.a{color:red;width:1px}.b{width:2px}.b,.c{color:red}`,
		}},
		{`@media print{.a{x:1}}@media print{.b{y:2}}.c{z:3}`, []string{
			`@media print{.a{x:1}}@media print{.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
			`@media print{.a{x:1}.b{y:2}}.c{z:3}`,
		}},
		{`@keyframes a{0%{x:1}}.b{y:1}@keyframes a{0%{x:2}}`, []string{
			`@keyframes a{0%{x:1}}.b{y:1}@keyframes a{0%{x:2}}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
			`@keyframes a{0%{x:2}}.b{y:1}`,
		}},
		{`.a{border-top-color:red;border:1px solid red}`, []string{
			`.a{border-top-color:red;border:1px solid red}`,
			`.a{border-top-color:red;border:1px solid red}`,
			`.a{border-top-color:red;border:1px solid red}`,
			`.a{border-top-color:red;border:1px solid red}`,
			`.a{border-top-color:red;border:1px solid red}`,
			`.a{border:1px solid red}`,
			`.a{border:1px solid red}`,
			`.a{border:1px solid red}`,
		}},
		{`.a{display:block}.a{display:flex}.b:hover{color:red}.c:hover{color:red}`, []string{
			`.a{display:block}.a{display:flex}.b:hover{color:red}.c:hover{color:red}`,
			`.a{display:block}.a{display:flex}.b:hover{color:red}.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover,.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover{color:red}.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover{color:red}.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover{color:red}.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover,.c:hover{color:red}`,
			`.a{display:block;display:flex}.b:hover,.c:hover{color:red}`,
		}},
		{`.a{x:1}.b{y:2}.a{x:1}`, []string{
			`.a{x:1}.b{y:2}.a{x:1}`,
			`.a{x:1}.b{y:2}.a{x:1}`,
			`.a{x:1}.b{y:2}.a{x:1}`,
			`.a{x:1}.b{y:2}.a{x:1}`,
			`.a{x:1}.b{y:2}.a{x:1}`,
			`.b{y:2}.a{x:1}`,
			`.b{y:2}.a{x:1}`,
			`.b{y:2}.a{x:1}`,
		}},
		{`a{color:red}a{color:red!important}a{color:blue}`, []string{
			`a{color:red}a{color:red!important}a{color:#00f}`,
			`a{color:red}a{color:red!important}a{color:#00f}`,
			`a{color:red;color:red!important;color:#00f}`,
			`a{color:red;color:red!important;color:#00f}`,
			`a{color:red;color:red!important;color:#00f}`,
			`a{color:red!important}`,
			`a{color:red!important}`,
			`a{color:red!important}`,
		}},
	}
	for _, tt := range tests {
		ast := prepared(t, tt.css)
		indexer, err := prepare(ast, nil)
		if err != nil {
			t.Fatalf("prepare %q: %v", tt.css, err)
		}
		steps := []struct {
			name string
			run  func() error
		}{
			{"prepare", func() error { return nil }},
			{"mergeAtrule", func() error { mergeAtrule(ast, false); return nil }},
			{"initialMergeRuleset", func() error { initialMergeRuleset(ast); return nil }},
			{"disjoinRuleset", func() error { disjoinRuleset(ast); return nil }},
			{"restructShorthand", func() error { restructShorthand(ast, indexer); return nil }},
			{"restructBlock", func() error { return restructBlock(ast) }},
			{"mergeRuleset", func() error { mergeRuleset(ast); return nil }},
			{"restructRuleset", func() error { restructRuleset(ast); return nil }},
		}
		for i, step := range steps {
			if err := step.run(); err != nil {
				t.Fatalf("%s %q: %v", step.name, tt.css, err)
			}
			if got := csstree.Generate(ast); got != tt.want[i] {
				t.Errorf("%q after %s = %q; want %q", tt.css, step.name, got, tt.want[i])
				break
			}
		}
	}
}

func TestRunForceMediaMerge(t *testing.T) {
	ast := prepared(t, "@media print{.a{color:red}}.b{y:2}@media print{.c{color:red}}")
	if err := Run(ast, Options{ForceMediaMerge: true}); err != nil {
		t.Fatal(err)
	}
	if got, want := csstree.Generate(ast), ".b{y:2}@media print{.a,.c{color:red}}"; got != want {
		t.Errorf("Run = %q; want %q", got, want)
	}
}

func TestRunNestedAtrule(t *testing.T) {
	ast := prepared(t, "a{@media x{b{c:d}} color:red}")
	err := Run(ast, Options{})
	if err == nil || err.Error() != "TypeError: Cannot read properties of undefined (reading '0')" {
		t.Errorf("Run error = %v", err)
	}
}

func TestSpecificity(t *testing.T) {
	tests := []struct {
		selectorList string
		want         []string
	}{
		{`*`, []string{`0,0,0`}},
		{`a`, []string{`0,0,1`}},
		{`.a`, []string{`0,1,0`}},
		{`#a`, []string{`1,0,0`}},
		{`a.b#c`, []string{`1,1,1`}},
		{`a:hover`, []string{`0,1,1`}},
		{`a::before`, []string{`0,0,2`}},
		{`a:before`, []string{`0,0,2`}},
		{`:not(.a,#b)`, []string{`1,0,0`}},
		{`:is(a,.b)`, []string{`0,1,0`}},
		{`:where(#a)`, []string{`0,0,0`}},
		{`:nth-child(2n of #a)`, []string{`1,1,0`}},
		{`:nth-child(2n)`, []string{`0,1,0`}},
		{`a b > c`, []string{`0,0,3`}},
		{`:has(> .x)`, []string{`0,1,0`}},
		{`svg|*`, []string{`0,0,0`}},
		{`:-moz-any(.a)`, []string{`0,1,0`}},
	}
	for _, tt := range tests {
		node, err := csstree.Parse(tt.selectorList, &csstree.ParseOptions{Context: csstree.ContextSelectorList})
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.selectorList, err)
		}
		var got []string
		for selector := range csstree.Children(node).All() {
			spec, err := specificity(selector)
			if err != nil {
				t.Fatalf("specificity(%q): %v", tt.selectorList, err)
			}
			got = append(got, fmt.Sprintf("%d,%d,%d", spec[0], spec[1], spec[2]))
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("specificity(%q) = %v; want %v", tt.selectorList, got, tt.want)
		}
	}
}
