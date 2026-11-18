package lexer

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// matchCase is a declaration recorded with css-tree 2.2.0 under Node 24 by
// testdata/gen-fixtures.mjs: the declaration source, the tokens css-tree's
// prepareTokens generated for its value (type, value, index of the emitting
// node in walk order or -1) and what lexer.matchDeclaration returned.
type matchCase struct {
	Decl     string `json:"d"`
	Mutation int    `json:"x"` // index of the Percentage replaced by a Number 0 (csso's Percentage pass), -1 for none
	Property string `json:"p"`
	// TypeName is set for lexer.matchType cases, whose Decl is a bare value.
	TypeName string  `json:"ty"`
	Tokens   [][]any `json:"t"`
	Matched  int     `json:"m"`
	// Iterations is -1 when css-tree reports none (matching not attempted).
	Iterations int    `json:"i"`
	Error      string `json:"e"`
	// MatchError holds a SyntaxMatchError's css, mismatchOffset,
	// mismatchLength, start offset/line/column, end offset/line/column and
	// syntax.
	MatchError []any `json:"em"`
	// Traces holds getTrace() of every value node in walk order, each entry
	// as "T:type", "P:property" or "K:keyword"; null when not found.
	Traces []*[]string `json:"r"`
}

func loadCases(t *testing.T, path string) []matchCase {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}

	var cases []matchCase
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var c matchCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return cases
}

// errorText renders an error as the fixtures record css-tree's: its name and
// raw message.
func errorText(err error) string {
	var refErr *SyntaxReferenceError
	var matchErr *SyntaxMatchError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &refErr):
		return "SyntaxReferenceError: " + refErr.Error()
	case errors.As(err, &matchErr):
		return "SyntaxMatchError: " + matchErr.Reason
	}
	return "Error: " + err.Error()
}

func traceText(trace []defsyntax.Node) *[]string {
	if trace == nil {
		return nil
	}
	out := []string{}
	for _, syntax := range trace {
		switch s := syntax.(type) {
		case *defsyntax.Type:
			out = append(out, "T:"+s.Name)
		case *defsyntax.Property:
			out = append(out, "P:"+s.Name)
		case *defsyntax.Keyword:
			out = append(out, "K:"+s.Name)
		}
	}
	return &out
}

func formatTrace(trace *[]string) string {
	if trace == nil {
		return "null"
	}
	return "[" + strings.Join(*trace, " ") + "]"
}

// checkResult compares a match result with the recorded one; nodes are the
// value nodes in walk order.
func checkResult(c *matchCase, res *MatchResult, nodes []csstree.Node) []string {
	var diffs []string
	diff := func(format string, args ...any) {
		diffs = append(diffs, fmt.Sprintf(format, args...))
	}

	if got := errorText(res.Error); got != c.Error {
		diff("error %q, want %q", got, c.Error)
	}
	if got := btoi(res.Matched != nil); got != c.Matched {
		diff("matched %d, want %d", got, c.Matched)
	}
	if c.Iterations >= 0 && res.Iterations != c.Iterations {
		diff("iterations %d, want %d", res.Iterations, c.Iterations)
	}

	var matchErr *SyntaxMatchError
	if c.MatchError != nil && errors.As(res.Error, &matchErr) {
		got := []any{matchErr.CSS, matchErr.MismatchOffset, matchErr.MismatchLength,
			matchErr.Loc.Start.Offset, matchErr.Loc.Start.Line, matchErr.Loc.Start.Column,
			matchErr.Loc.End.Offset, matchErr.Loc.End.Line, matchErr.Loc.End.Column, matchErr.Syntax}
		if fmt.Sprint(got) != fmt.Sprint(c.MatchError) {
			diff("match error %v, want %v", got, c.MatchError)
		}
	}

	if len(nodes) != len(c.Traces) {
		diff("%d value nodes, want %d", len(nodes), len(c.Traces))
		return diffs
	}
	for i, node := range nodes {
		got := traceText(res.GetTrace(node))
		if formatTrace(got) != formatTrace(c.Traces[i]) {
			diff("node %d (%s) trace %s, want %s", i, node.Type(), formatTrace(got), formatTrace(c.Traces[i]))
		}
	}
	return diffs
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestMatchRecordedTokens replays the recorded token streams through the
// matcher, independently of the parser and generator.
func TestMatchRecordedTokens(t *testing.T) {
	l := Default()
	cases := loadCases(t, "testdata/cases.json.gz")
	failed := 0

	for i := range cases {
		c := &cases[i]

		nodes := make([]csstree.Node, len(c.Traces))
		for j := range nodes {
			nodes[j] = &csstree.Identifier{Name: fmt.Sprint(j)}
		}
		tokens := make([]token, len(c.Tokens))
		for j, raw := range c.Tokens {
			tokens[j] = token{typ: csstree.TokenType(raw[0].(float64)), value: raw[1].(string)}
			if idx := int(raw[2].(float64)); idx >= 0 {
				tokens[j].node = nodes[idx]
			}
		}

		// the first node in walk order is the value itself
		var res *MatchResult
		if c.TypeName != "" {
			if descriptor := l.Type(c.TypeName); descriptor == nil {
				res = failedMatch(&SyntaxReferenceError{Kind: "Unknown type", Reference: c.TypeName})
			} else {
				res = l.matchSyntax(descriptor, tokens, nodes[0], false)
			}
		} else if descriptor, err := l.matchableProperty(c.Property); err != nil {
			res = failedMatch(err)
		} else {
			res = l.matchSyntax(descriptor, tokens, nodes[0], true)
		}

		if diffs := checkResult(c, res, nodes); len(diffs) > 0 {
			failed++
			if failed <= 20 {
				t.Errorf("%s (mutation %d):\n  %s", c.Decl, c.Mutation, strings.Join(diffs, "\n  "))
			}
		}
	}

	t.Logf("%d/%d recorded cases match", len(cases)-failed, len(cases))
}

// valueNodes returns the nodes of a declaration value in walk order.
func valueNodes(value csstree.Node) []csstree.Node {
	var nodes []csstree.Node
	csstree.Walk(value, csstree.WalkOptions{Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
		nodes = append(nodes, node)
		return csstree.WalkContinue
	}})
	return nodes
}

// replacePercentage replaces the n-th Percentage of the value (in walk
// order) with a Number 0, as csso's Percentage pass does before asking the
// lexer whether the number is a length.
func replacePercentage(value csstree.Node, n int) {
	seen := 0
	csstree.Walk(value, csstree.WalkOptions{Visit: "Percentage", Enter: func(_ *csstree.WalkContext, node csstree.Node, item *csstree.ListItem, _ *csstree.List) csstree.WalkAction {
		if seen == n && item != nil {
			item.Data = &csstree.Number{Value: "0"}
		}
		seen++
		return csstree.WalkContinue
	}})
}

// TestMatchDeclarations parses the recorded declarations and matches them
// end to end: parsing, token generation and matching.
func TestMatchDeclarations(t *testing.T) {
	l := Default()
	cases := loadCases(t, "testdata/cases.json.gz")
	failed := 0

	for i := range cases {
		c := &cases[i]

		var value csstree.Node
		var res func() *MatchResult
		if c.TypeName != "" {
			ast, err := csstree.Parse(c.Decl, &csstree.ParseOptions{Context: csstree.ContextValue})
			if err != nil {
				t.Fatalf("%s: %v", c.Decl, err)
			}
			value = ast
			res = func() *MatchResult { return l.MatchType(c.TypeName, value) }
		} else {
			ast, err := csstree.Parse("a{"+c.Decl+"}", nil)
			if err != nil {
				t.Fatalf("%s: %v", c.Decl, err)
			}
			decl, ok := csstree.Find(ast, func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) bool {
				return node.Type() == "Declaration"
			}).(*csstree.Declaration)
			if !ok {
				t.Fatalf("%s: no declaration", c.Decl)
			}
			if c.Mutation >= 0 {
				replacePercentage(decl.Value, c.Mutation)
			}
			value = decl.Value
			res = func() *MatchResult { return l.MatchDeclaration(decl) }
		}

		nodes := valueNodes(value)
		var diffs []string
		if got, want := generatedTokenText(prepareTokens(value), nodes), recordedTokenText(c); got != want {
			diffs = append(diffs, fmt.Sprintf("tokens %s, want %s", got, want))
		}
		diffs = append(diffs, checkResult(c, res(), nodes)...)

		if len(diffs) > 0 {
			failed++
			if failed <= 20 {
				t.Errorf("%s (mutation %d):\n  %s", c.Decl, c.Mutation, strings.Join(diffs, "\n  "))
			}
		}
	}

	t.Logf("%d/%d declarations match", len(cases)-failed, len(cases))
}

// recordedTokenText renders the recorded tokens as type:"value"@node.
func recordedTokenText(c *matchCase) string {
	parts := make([]string, len(c.Tokens))
	for i, raw := range c.Tokens {
		parts[i] = fmt.Sprintf("%v:%q@%v", raw[0], raw[1], raw[2])
	}
	return strings.Join(parts, " ")
}

// generatedTokenText renders tokens like recordedTokenText, identifying
// nodes by their walk order index.
func generatedTokenText(tokens []token, nodes []csstree.Node) string {
	parts := make([]string, len(tokens))
	for i, tok := range tokens {
		parts[i] = fmt.Sprintf("%d:%q@%d", tok.typ, tok.value, slices.Index(nodes, tok.node))
	}
	return strings.Join(parts, " ")
}

// TestCssoQueries covers the two questions csso asks: whether an identifier
// is a color (lib/replace/color.js) and whether a zero that replaced a
// percentage is still a length (lib/replace/Percentage.js).
func TestCssoQueries(t *testing.T) {
	l := Default()

	for _, tc := range []struct {
		decl      string
		pick      string // type of the node queried, its first occurrence
		percentTo bool   // replace the first Percentage with a Number 0 first
		typeName  string
		want      bool
		wantErr   error
	}{
		{decl: "color: red", pick: "Identifier", typeName: "color", want: true},
		{decl: "border-color: Red", pick: "Identifier", typeName: "color", want: true},
		{decl: "background: red url(a.png)", pick: "Identifier", typeName: "color", want: true},
		{decl: "font-family: red", pick: "Identifier", typeName: "color", want: false},
		{decl: "color: inherit", pick: "Identifier", typeName: "color", want: false},
		{decl: "color: red var(--a)", pick: "Identifier", typeName: "color", wantErr: ErrVarNotSupported},
		{decl: "--x: red", pick: "Raw", typeName: "color", wantErr: ErrCustomProperty},
		{decl: "margin: 0%", pick: "Number", percentTo: true, typeName: "length", want: true},
		{decl: "line-height: 0%", pick: "Number", percentTo: true, typeName: "length", want: false}, // <number> wins
		{decl: "transform: translate(0%, 1px)", pick: "Number", percentTo: true, typeName: "length", want: true},
	} {
		ast, err := csstree.Parse("a{"+tc.decl+"}", nil)
		if err != nil {
			t.Fatal(err)
		}
		decl := csstree.Find(ast, func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) bool {
			return node.Type() == "Declaration"
		}).(*csstree.Declaration)
		if tc.percentTo {
			replacePercentage(decl.Value, 0)
		}
		node := csstree.Find(decl, func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem, _ *csstree.List) bool {
			return node.Type() == tc.pick
		})

		res := l.MatchDeclaration(decl)
		if tc.wantErr != nil && !errors.Is(res.Error, tc.wantErr) {
			t.Errorf("%s: error %v, want %v", tc.decl, res.Error, tc.wantErr)
		}
		if got := res.IsType(node, tc.typeName); got != tc.want {
			t.Errorf("%s: IsType(%s, %s) = %v, want %v", tc.decl, tc.pick, tc.typeName, got, tc.want)
		}
	}
}
