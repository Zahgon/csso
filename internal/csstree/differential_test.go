package csstree

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The fixture testdata/parse.json.gz is recorded from css-tree 2.2.0 as
// forked by csso 5.0.5, running on Node.js 24. For every input it holds the
// expected tokens, AST, recovered and fatal parse errors, generated CSS,
// generated tokens, source map and walk traces (large values as truncated
// SHA-1 digests of a canonical dump).

type caseOptions struct {
	Context             string `json:"context"`
	Positions           bool   `json:"positions"`
	Filename            string `json:"filename"`
	Atrule              string `json:"atrule"`
	ParseValue          *bool  `json:"parseValue"`
	ParseAtrulePrelude  *bool  `json:"parseAtrulePrelude"`
	ParseRulePrelude    *bool  `json:"parseRulePrelude"`
	ParseCustomProperty bool   `json:"parseCustomProperty"`
	Offset              int    `json:"offset"`
	Line                int    `json:"line"`
	Column              int    `json:"column"`
}

type recordedCase struct {
	CSS       string      `json:"css"`
	Options   caseOptions `json:"o"`
	Tokens    string      `json:"tokens"`
	Error     string      `json:"error"`
	Formatted string      `json:"formatted"`
	Recovered string      `json:"recovered"`
	AST       string      `json:"ast"`
	CSS2      *string     `json:"css2"`
	GenTokens string      `json:"gentokens"`
	Map       string      `json:"map"`
	Walk      string      `json:"walk"`
	Comments  string      `json:"comments"`
	Mutate    string      `json:"mutate"`
}

type recordedName struct {
	Name     string `json:"name"`
	Keyword  []any  `json:"keyword"`
	Property []any  `json:"property"`
}

type recordedEscape struct {
	S            string  `json:"s"`
	String       *string `json:"string"`
	StringApos   *string `json:"stringApos"`
	URL          *string `json:"url"`
	Ident        *string `json:"ident"`
	DecodeString *string `json:"decodeString"`
	DecodeURL    *string `json:"decodeUrl"`
	DecodeIdent  *string `json:"decodeIdent"`
}

type fixture struct {
	Cases   []recordedCase   `json:"cases"`
	Names   []recordedName   `json:"names"`
	Escapes []recordedEscape `json:"escapes"`
}

func loadFixture(t *testing.T) *fixture {
	t.Helper()
	var fx fixture
	loadFixtureInto(t, &fx)
	return &fx
}

func loadFixtureInto(t *testing.T, v any) {
	t.Helper()
	f, err := os.Open("testdata/parse.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func digest(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

func (o caseOptions) parseOptions(onError func(*SyntaxError, Node), onComment func(string, *Location)) *ParseOptions {
	return &ParseOptions{
		Context:             o.Context,
		Atrule:              o.Atrule,
		Positions:           o.Positions,
		Filename:            o.Filename,
		Offset:              o.Offset,
		Line:                o.Line,
		Column:              o.Column,
		RawValue:            o.ParseValue != nil && !*o.ParseValue,
		RawAtrulePrelude:    o.ParseAtrulePrelude != nil && !*o.ParseAtrulePrelude,
		RawRulePrelude:      o.ParseRulePrelude != nil && !*o.ParseRulePrelude,
		ParseCustomProperty: o.ParseCustomProperty,
		OnParseError:        onError,
		OnComment:           onComment,
	}
}

func errorLine(e *SyntaxError) string {
	return fmt.Sprintf("%s@%d,%d,%d", e.Message, e.Offset, e.Line, e.Column)
}

func TestDifferential(t *testing.T) {
	fx := loadFixture(t)
	failures := 0
	fail := func(i int, c recordedCase, format string, args ...any) {
		failures++
		if failures <= 30 {
			t.Errorf("case %d %q %+v: %s", i, c.CSS, c.Options, fmt.Sprintf(format, args...))
		}
	}

	for i, c := range fx.Cases {
		var toks []string
		for _, tok := range Tokenize(c.CSS) {
			toks = append(toks, fmt.Sprintf("%d:%d:%d", tok.Type, tok.Start, tok.End))
		}
		if got := digest(strings.Join(toks, " ")); got != c.Tokens {
			fail(i, c, "tokens differ")
		}

		var recovered, comments []string
		ast, err := Parse(c.CSS, c.Options.parseOptions(func(e *SyntaxError, _ Node) {
			recovered = append(recovered, errorLine(e))
		}, func(value string, loc *Location) {
			comments = append(comments, quoteDump(value)+"@"+dumpLocation(loc))
		}))
		if len(comments) > 0 || c.Comments != "" {
			if got := digest(strings.Join(comments, " ")); got != c.Comments {
				fail(i, c, "comments differ: %q", comments)
			}
		}
		if got := strings.Join(recovered, "\n"); got != c.Recovered {
			fail(i, c, "recovered errors:\n got %q\nwant %q", got, c.Recovered)
		}
		if err != nil {
			syntaxErr, ok := err.(*SyntaxError)
			if !ok {
				fail(i, c, "unexpected error %v", err)
				continue
			}
			if got := errorLine(syntaxErr); got != c.Error {
				fail(i, c, "error: got %q want %q", got, c.Error)
			}
			if got := digest(syntaxErr.FormattedMessage()); got != c.Formatted {
				fail(i, c, "formatted message differs:\n%s", syntaxErr.FormattedMessage())
			}
			continue
		}
		if c.Error != "" {
			fail(i, c, "expected error %q", c.Error)
			continue
		}

		if got := digest(dumpNode(ast)); got != c.AST {
			fail(i, c, "AST differs: %s", dumpNode(ast))
		}
		if got := Generate(ast); c.CSS2 == nil || got != *c.CSS2 {
			fail(i, c, "generate: got %q want %v", got, *c.CSS2)
		}
		if got := digest(generatedTokensTrace(ast)); got != c.GenTokens {
			fail(i, c, "generated tokens differ")
		}
		if c.Options.Positions {
			css, sourceMap := GenerateWithSourceMap(ast)
			filename := c.Options.Filename
			if filename == "" {
				filename = "<unknown>"
			}
			sourceMap.File = filename
			sourceMap.SetSourceContent(filename, c.CSS)
			if css != *c.CSS2 {
				fail(i, c, "source map css differs")
			}
			if got := digest(sourceMap.String()); got != c.Map {
				fail(i, c, "source map differs: %s", sourceMap.String())
			}
		}
		if c.Walk != "" {
			trace := strings.Join([]string{
				walkTrace(ast, WalkOptions{}),
				walkTrace(ast, WalkOptions{Reverse: true}),
				walkTrace(ast, WalkOptions{Visit: "Rule"}),
				walkTrace(ast, WalkOptions{Visit: "Declaration", Reverse: true}),
				walkTrace(ast, WalkOptions{Visit: "Atrule"}),
				walkTrace(ast, WalkOptions{Visit: "Selector"}),
			}, "\n")
			if got := digest(trace); got != c.Walk {
				fail(i, c, "walk trace differs: %s", trace)
			}
		}
		if cloned := Clone(ast); dumpNode(cloned) != dumpNode(ast) {
			fail(i, c, "clone differs")
		}
		if c.Mutate != "" {
			if got := digest(mutatingWalk(ast, i%8 == 0)); got != c.Mutate {
				fail(i, c, "mutating walk differs: %s", mutatingWalk(ast, i%8 == 0))
			}
		}
	}

	if failures > 0 {
		t.Errorf("%d of %d checks failed", failures, len(fx.Cases))
	}
}

func TestNames(t *testing.T) {
	for _, n := range loadFixture(t).Names {
		k := Keyword(n.Name)
		gotK := []any{k.Basename, k.Name, k.Prefix, k.Vendor, k.Custom}
		if fmt.Sprint(gotK) != fmt.Sprint(n.Keyword) {
			t.Errorf("Keyword(%q) = %v, want %v", n.Name, gotK, n.Keyword)
		}
		p := Property(n.Name)
		gotP := []any{p.Basename, p.Name, p.Hack, p.Vendor, p.Prefix, p.Custom}
		if fmt.Sprint(gotP) != fmt.Sprint(n.Property) {
			t.Errorf("Property(%q) = %v, want %v", n.Name, gotP, n.Property)
		}
	}
}

func TestEscapes(t *testing.T) {
	check := func(name, input string, got string, want *string) {
		if want != nil && got != *want {
			t.Errorf("%s(%q) = %q, want %q", name, input, got, *want)
		}
	}
	for _, e := range loadFixture(t).Escapes {
		check("EncodeString", e.S, EncodeString(e.S, false), e.String)
		check("EncodeString'", e.S, EncodeString(e.S, true), e.StringApos)
		check("EncodeURL", e.S, EncodeURL(e.S), e.URL)
		check("EncodeIdent", e.S, EncodeIdent(e.S), e.Ident)
		check("DecodeString", e.S, DecodeString(e.S), e.DecodeString)
		check("DecodeURL", e.S, DecodeURL(e.S), e.DecodeURL)
		check("DecodeIdent", e.S, DecodeIdent(e.S), e.DecodeIdent)
	}
}

// dumpNode renders a tree in the canonical form used by the recorder:
// node fields in css-tree's structure order, strings quoted.
func dumpNode(n Node) string {
	var b strings.Builder
	writeDump(&b, n)
	return b.String()
}

func writeDump(b *strings.Builder, n Node) {
	if n == nil {
		b.WriteString("null")
		return
	}
	b.WriteString("(" + n.Type() + "@" + dumpLocation(n.Loc()))
	str := func(key, v string) { b.WriteString(" " + key + "=" + quoteDump(v)) }
	nullable := func(key, v string) {
		if v == "" {
			b.WriteString(" " + key + "=null")
		} else {
			str(key, v)
		}
	}
	node := func(key string, v Node) {
		b.WriteString(" " + key + "=")
		writeDump(b, v)
	}
	list := func(key string, l *List) {
		b.WriteString(" " + key + "=")
		if l == nil {
			b.WriteString("null")
			return
		}
		b.WriteString("[")
		for it := l.Head; it != nil; it = it.Next {
			if it != l.Head {
				b.WriteString(" ")
			}
			writeDump(b, it.Data)
		}
		b.WriteString("]")
	}

	switch n := n.(type) {
	case *AnPlusB:
		nullable("a", n.A)
		nullable("b", n.B)
	case *Atrule:
		str("name", n.Name)
		node("prelude", n.Prelude)
		node("block", blockField(n.Block))
	case *AttributeSelector:
		node("name", n.Name)
		nullable("matcher", n.Matcher)
		node("value", n.Value)
		nullable("flags", n.Flags)
	case *Combinator:
		str("name", n.Name)
	case *Comment:
		str("value", n.Value)
	case *Declaration:
		switch n.Important {
		case "":
			b.WriteString(" important=false")
		case "important":
			b.WriteString(" important=true")
		default:
			str("important", n.Important)
		}
		str("property", n.Property)
		node("value", n.Value)
	case *Dimension:
		str("value", n.Value)
		str("unit", n.Unit)
	case *Function:
		str("name", n.Name)
		list("children", n.Children)
	case *Hash:
		str("value", n.Value)
	case *IdSelector:
		str("name", n.Name)
	case *ClassSelector:
		str("name", n.Name)
	case *Identifier:
		str("name", n.Name)
	case *MediaFeature:
		str("name", n.Name)
		node("value", n.Value)
	case *Nth:
		node("nth", n.Nth)
		if n.Selector == nil {
			b.WriteString(" selector=null")
		} else {
			node("selector", n.Selector)
		}
	case *Number:
		str("value", n.Value)
	case *Operator:
		str("value", n.Value)
	case *Percentage:
		str("value", n.Value)
	case *PseudoClassSelector:
		str("name", n.Name)
		list("children", n.Children)
	case *PseudoElementSelector:
		str("name", n.Name)
		list("children", n.Children)
	case *Ratio:
		str("left", n.Left)
		str("right", n.Right)
	case *Raw:
		str("value", n.Value)
	case *Rule:
		node("prelude", n.Prelude)
		node("block", blockField(n.Block))
	case *String:
		str("value", n.Value)
	case *TypeSelector:
		str("name", n.Name)
	case *UnicodeRange:
		str("value", n.Value)
	case *Url:
		str("value", n.Value)
	case *WhiteSpace:
		str("value", n.Value)
	case *CDO, *CDC:
	default:
		list("children", Children(n))
	}
	b.WriteString(")")
}

func dumpLocation(loc *Location) string {
	if loc == nil {
		return "-"
	}
	return fmt.Sprintf("%s:%d,%d,%d-%d,%d,%d", loc.Source,
		loc.Start.Offset, loc.Start.Line, loc.Start.Column,
		loc.End.Offset, loc.End.Line, loc.End.Column)
}

// mutatingWalk walks a clone of root while inserting, removing and
// replacing list items from the callbacks, mirroring the recorder.
func mutatingWalk(root Node, reverse bool) string {
	copied := Clone(root)
	var trace []string
	count := 0
	Walk(copied, WalkOptions{
		Reverse: reverse,
		Enter: func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
			count++
			entry := node.Type()
			if ctx.Rule != nil {
				entry += "R"
			}
			if ctx.Declaration != nil {
				entry += "D"
			}
			trace = append(trace, entry)
			if item != nil && count%7 == 0 {
				list.InsertData(&Raw{Value: "R" + strconv.Itoa(count)}, item)
			}
			if item != nil && count%3 == 0 {
				list.Remove(item)
				return WalkSkip
			}
			if count%11 == 0 && item != nil && item.Next != nil {
				list.Remove(item.Next)
			}
			return WalkContinue
		},
		Leave: func(_ *WalkContext, node Node, item *ListItem, list *List) WalkAction {
			trace = append(trace, "/"+node.Type())
			if item != nil && count%5 == 0 {
				list.Replace(item, NewListItem(&Raw{Value: "L" + strconv.Itoa(count)}))
			}
			return WalkContinue
		},
	})
	return strings.Join(trace, " ") + "\n" + Generate(copied)
}

func quoteDump(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func walkTrace(root Node, opts WalkOptions) string {
	var out []string
	ctxFlags := func(c *WalkContext) string {
		var b strings.Builder
		for _, f := range []struct {
			set  bool
			flag string
		}{
			{c.Stylesheet != nil, "S"}, {c.Atrule != nil, "A"}, {c.AtrulePrelude != nil, "P"},
			{c.Rule != nil, "R"}, {c.Selector != nil, "L"}, {c.Block != nil, "B"},
			{c.Declaration != nil, "D"}, {c.Function != nil, "F"},
		} {
			if f.set {
				b.WriteString(f.flag)
			}
		}
		return b.String()
	}
	opts.Enter = func(ctx *WalkContext, node Node, item *ListItem, list *List) WalkAction {
		s := ">" + node.Type() + ":" + ctxFlags(ctx)
		if item != nil {
			s += "i"
		}
		if list != nil {
			s += "l"
		}
		out = append(out, s)
		return WalkContinue
	}
	opts.Leave = func(ctx *WalkContext, node Node, _ *ListItem, _ *List) WalkAction {
		out = append(out, "<"+node.Type()+":"+ctxFlags(ctx))
		return WalkContinue
	}
	Walk(root, opts)
	return strings.Join(out, " ")
}

func generatedTokensTrace(root Node) string {
	var out []string
	for _, tok := range GenerateTokens(root) {
		owner := "-"
		if tok.Node != nil {
			owner = tok.Node.Type()
		}
		out = append(out, fmt.Sprintf("%d:%s:%s", tok.Type, tok.Value, owner))
	}
	return strings.Join(out, "\u0001")
}
