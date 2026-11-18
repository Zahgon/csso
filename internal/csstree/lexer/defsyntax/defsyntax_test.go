package defsyntax

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// syntaxCase is a definition recorded with css-tree 2.2.0 under Node 24 by
// ../testdata/gen-fixtures.mjs:
// every syntax of css-tree's lexer data plus hand-written edge cases, with
// the generated forms and JSON.stringify of the parsed tree, or the error
// message of a malformed definition.
type syntaxCase struct {
	Source      string `json:"s"`
	Generated   string `json:"g"`
	Compact     string `json:"c"`
	ForceBraces string `json:"f"`
	AST         string `json:"a"`
	Err         string `json:"err"`
}

// jsonString quotes s as JSON.stringify does (no HTML escaping).
func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func boundJSON(b *RangeBound) string {
	switch {
	case b == nil:
		return "null"
	case b.Unit != "":
		return jsonString(b.String())
	}
	return b.String()
}

// astJSON serializes a tree the way JSON.stringify serializes css-tree's.
func astJSON(node Node) string {
	switch n := node.(type) {
	case *Group:
		terms := make([]string, len(n.Terms))
		for i, term := range n.Terms {
			terms[i] = astJSON(term)
		}
		return `{"type":"Group","terms":[` + strings.Join(terms, ",") + `],"combinator":` + jsonString(n.Combinator) +
			`,"disallowEmpty":` + strconv.FormatBool(n.DisallowEmpty) + `,"explicit":` + strconv.FormatBool(n.Explicit) + `}`
	case *Multiplier:
		return `{"type":"Multiplier","comma":` + strconv.FormatBool(n.Comma) + `,"min":` + strconv.Itoa(n.Min) +
			`,"max":` + strconv.Itoa(n.Max) + `,"term":` + astJSON(n.Term) + `}`
	case *Type:
		opts := "null"
		if n.Opts != nil {
			opts = `{"type":"Range","min":` + boundJSON(n.Opts.Min) + `,"max":` + boundJSON(n.Opts.Max) + `}`
		}
		return `{"type":"Type","name":` + jsonString(n.Name) + `,"opts":` + opts + `}`
	case *Property, *Keyword, *AtKeyword, *Function:
		var name string
		switch n := n.(type) {
		case *Property:
			name = n.Name
		case *Keyword:
			name = n.Name
		case *AtKeyword:
			name = n.Name
		case *Function:
			name = n.Name
		}
		return `{"type":"` + node.Kind() + `","name":` + jsonString(name) + `}`
	case *String:
		return `{"type":"String","value":` + jsonString(n.Value) + `}`
	case *Token:
		return `{"type":"Token","value":` + jsonString(n.Value) + `}`
	case *Comma:
		return `{"type":"Comma"}`
	}
	panic("unexpected node")
}

func TestParseGenerate(t *testing.T) {
	f, err := os.Open("testdata/syntax.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var c syntaxCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		count++

		ast, err := Parse(c.Source)
		if c.Err != "" {
			if err == nil || err.Error() != c.Err {
				t.Errorf("Parse(%q) error = %v, want %q", c.Source, err, c.Err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): %v", c.Source, err)
			continue
		}

		if got := astJSON(ast); got != c.AST {
			t.Errorf("Parse(%q) =\n  %s\nwant\n  %s", c.Source, got, c.AST)
		}
		if got := Generate(ast, GenerateOptions{}); got != c.Generated {
			t.Errorf("Generate(%q) = %q, want %q", c.Source, got, c.Generated)
		}
		if got := Generate(ast, GenerateOptions{Compact: true}); got != c.Compact {
			t.Errorf("Generate(%q, compact) = %q, want %q", c.Source, got, c.Compact)
		}
		if got := Generate(ast, GenerateOptions{ForceBraces: true}); got != c.ForceBraces {
			t.Errorf("Generate(%q, forceBraces) = %q, want %q", c.Source, got, c.ForceBraces)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d definitions checked", count)
}

func TestGenerateDecorate(t *testing.T) {
	ast, err := Parse("<length [0,∞]>{1,2} | auto")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	got := Generate(ast, GenerateOptions{Decorate: func(text string, node any) string {
		switch n := node.(type) {
		case Node:
			kinds = append(kinds, n.Kind())
		case *Range:
			kinds = append(kinds, "Range")
		}
		return "(" + text + ")"
	}})
	// a multiplier decorates only its suffix; its term is decorated on its own
	want := "((<length( [0,∞])>)({1,2}) | (auto))"
	if got != want {
		t.Errorf("Generate with Decorate = %q, want %q", got, want)
	}
	if strings.Join(kinds, " ") != "Range Type Multiplier Keyword Group" {
		t.Errorf("decorated kinds = %v", kinds)
	}
}

func TestWalk(t *testing.T) {
	ast, err := Parse("a [ <b> | c( ) ]#")
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	Walk(ast, func(n Node) { events = append(events, "+"+n.Kind()) }, func(n Node) { events = append(events, "-"+n.Kind()) })
	// `c( )` is a Function and a Token juxtaposed, which binds tighter than |
	want := "+Group +Keyword -Keyword +Multiplier +Group +Type -Type +Group +Function -Function +Token -Token -Group -Group -Multiplier -Group"
	if got := strings.Join(events, " "); got != want {
		t.Errorf("Walk events = %s, want %s", got, want)
	}
}
