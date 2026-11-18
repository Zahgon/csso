package csso_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/css/csso-go"
)

// The expectations below were recorded from csso 5.0.5 on Node.js 24.

// errorString renders err like the JavaScript `String(e.name) + ': ' +
// e.message` of the exception csso throws; "" for no error.
func errorString(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var cssoErr *csso.Error
	if !errors.As(err, &cssoErr) {
		t.Fatalf("error %v (%T) is not a *csso.Error", err, err)
	}
	return cssoErr.Error()
}

func TestMinify(t *testing.T) {
	tests := []struct {
		css     string
		opts    csso.Options
		want    string
		wantErr string
	}{
		{`.a { color: #ff0000; } .b { color: red }`, csso.Options{}, `.a,.b{color:red}`, ""},
		{`a{margin:0px 0px 0px 0px;color:rgba(0,0,0,.5)}`, csso.Options{}, `a{margin:0;color:rgba(0,0,0,.5)}`, ""},
		{`/*! c */ a{x:1} /*! d */ b{y:2}`, csso.Options{}, "/*! c */\na{x:1}\n/*! d */\nb{y:2}", ""},
		{`/*! c */ a{x:1} /*! d */ b{y:2}`, csso.Options{Comments: csso.CommentsFirstExclamation}, "/*! c */\na{x:1}b{y:2}", ""},
		{`/*! c */ a{x:1} /*! d */ b{y:2}`, csso.Options{Comments: csso.CommentsNone}, `a{x:1}b{y:2}`, ""},
		{`/* plain */ a{x:1}`, csso.Options{Comments: csso.CommentsExclamation}, `a{x:1}`, ""},
		{`.a{color:red}.b{color:red}`, csso.Options{Restructure: new(false)}, `.a{color:red}.b{color:red}`, ""},
		{`@media print{.a{x:1}}.b{y:2}@media print{.c{z:3}}`, csso.Options{ForceMediaMerge: true}, `.b{y:2}@media print{.a{x:1}.c{z:3}}`, ""},
		{`@media print{.a{x:1}}.b{y:2}@media print{.c{z:3}}`, csso.Options{}, `@media print{.a{x:1}}.b{y:2}@media print{.c{z:3}}`, ""},
		{`a{color:red}.b{color:red}#c{color:red}`, csso.Options{Usage: &csso.Usage{Tags: []string{`a`}, Classes: []string{}}}, `#c,a{color:red}`, ""},
		{`a{color:red}.b{color:red}#c{color:red}`, csso.Options{Usage: &csso.Usage{Blacklist: &csso.UsageBlacklist{IDs: []string{`c`}}}}, `.b,a{color:red}`, ""},
		{`.a{color:red}.b{color:blue}.a{margin:0}`, csso.Options{Usage: &csso.Usage{Scopes: [][]string{{`a`}, {`b`}}}}, `.a{color:red;margin:0}.b{color:#00f}`, ""},
		{`.a.b{color:red}`, csso.Options{Usage: &csso.Usage{Scopes: [][]string{{`a`}, {`b`}}}}, "", `Error: Selector can't has classes from different scopes: .a.b`},
		{`a{color:red}`, csso.Options{Usage: &csso.Usage{Scopes: [][]string{{`a`}, {`a`}}}}, "", `Error: Class can't be used for several scopes: a`},
		{`a{color:red}`, csso.Options{Usage: &csso.Usage{Scopes: [][]string{{`a`}, nil}}}, "", `Error: Wrong usage format`},
		{`a:not{color:red}`, csso.Options{}, "", `TypeError: Cannot read properties of null (reading 'first')`},
		{`a:nth-child{x:1}`, csso.Options{}, "", `TypeError: Cannot read properties of null (reading 'first')`},
		{`a{@media x{b{c:d}} color:red}`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading '0')`},
		{`@charset "utf-8";@import url(a.css);a{x:1}@import url(b.css);`, csso.Options{}, `@charset "utf-8";@import url(a.css);a{x:1}`, ""},
		{`a{x:1}@charset "utf-8";`, csso.Options{}, `a{x:1}`, ""},
		{`@keyframes k{from{top:0}50%{top:10px}100%{top:20px}}@keyframes k{0%{a:1}}`, csso.Options{}, `@keyframes k{0%{a:1}}`, ""},
		{`@keyframes k{@foo bar;}`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading 'forEach')`},
		{`a{color:hsla(120,100%,50%,.3);background:rgb(100%,0%,0%);border-color:hsl(0,0%,0%)}`, csso.Options{}, `a{color:rgba(0,255,0,.3);background:red;border-color:#000}`, ""},
		{`a{background:linear-gradient(rgba(255,0,0,0),#fff)}`, csso.Options{}, `a{background:linear-gradient(rgba(255,0,0,0),#fff)}`, ""},
		{`a{font:normal bold 12px/normal Arial;font-weight:normal}`, csso.Options{}, `a{font:700 12px Arial;font-weight:400}`, ""},
		{`a{background:none repeat scroll 0 0 transparent;border:none;outline:1px none red}`, csso.Options{}, `a{background:0 0;border:0;outline:1px red}`, ""},
		{`a{width:0%;margin:0%;flex:0px;height:calc(0px + 1px)}`, csso.Options{}, `a{width:0%;margin:0;flex:0px;height:calc(0px + 1px)}`, ""},
		{`a{margin-top:1px;margin-right:2px;margin-bottom:3px;margin-left:2px}`, csso.Options{}, `a{margin:1px 2px 3px}`, ""},
		{`a{padding:1px\9;padding-top:2px\9}`, csso.Options{}, `a{padding:2px\9 1px\9 1px\9}`, ""},
		{`a{margin:1px!important;margin-top:2px}`, csso.Options{}, `a{margin:1px!important}`, ""},
		{`a{color:red;color:rgba(1,2,3,.5)}b{color:red}`, csso.Options{}, `a{color:rgba(1,2,3,.5)}a,b{color:red}`, ""},
		{`.a{color:red;margin:0}.b{color:red;padding:0}`, csso.Options{}, `.a,.b{color:red}.a{margin:0}.b{padding:0}`, ""},
		{`input[type="text"],a[data-x="a b"],a[x="-1"]{x:y}`, csso.Options{}, `a[data-x="a b"],a[x="-1"],input[type=text]{x:y}`, ""},
		{`*.a,*:hover,*|*.b,* *{x:y}`, csso.Options{}, `* *,*|*.b,.a,:hover{x:y}`, ""},
		{`a{--x:  value  ;--y: ;color:var(--x)}`, csso.Options{}, `a{--x:value;--y: ;color:var(--x)}`, ""},
		{`a{content:"a\"b";background:url( "a b.png" )}`, csso.Options{}, `a{content:'a"b';background:url(a\ b.png)}`, ""},
		{`a{color:#AbCdEf;background:#aabbcc;border-color:#000080}`, csso.Options{}, `a{color:#abcdef;background:#abc;border-color:navy}`, ""},
		{`a{display:flex;display:-webkit-box}b{cursor:pointer;cursor:hand}`, csso.Options{}, `a{display:flex;display:-webkit-box}b{cursor:pointer;cursor:hand}`, ""},
	}
	for _, tt := range tests {
		got, err := csso.Minify(tt.css, tt.opts)
		if gotErr := errorString(t, err); got.CSS != tt.want || gotErr != tt.wantErr {
			t.Errorf("Minify(%q, %+v) = %q, %q; want %q, %q", tt.css, tt.opts, got.CSS, gotErr, tt.want, tt.wantErr)
		}
		if got.Map != nil {
			t.Errorf("Minify(%q) returned a source map without SourceMap", tt.css)
		}
	}
}

func TestMinifySourceMap(t *testing.T) {
	tests := []struct {
		css     string
		opts    csso.Options
		want    string
		wantMap string
	}{
		{".a{color:red}\n.b{color:blue}", csso.Options{SourceMap: true, Filename: `in.css`}, `.a{color:red}.b{color:#00f}`, `{"version":3,"sources":["in.css"],"names":[],"mappings":"AAAA,E,CAAG,S,CACH,E,CAAG,U","file":"in.css","sourcesContent":[".a{color:red}\n.b{color:blue}"]}`},
		{`a{color:red}`, csso.Options{SourceMap: true}, `a{color:red}`, `{"version":3,"sources":["<unknown>"],"names":[],"mappings":"AAAA,C,CAAE,S","file":"<unknown>","sourcesContent":["a{color:red}"]}`},
	}
	for _, tt := range tests {
		got, err := csso.Minify(tt.css, tt.opts)
		if err != nil {
			t.Fatalf("Minify(%q): %v", tt.css, err)
		}
		if got.CSS != tt.want || got.Map == nil || got.Map.String() != tt.wantMap {
			t.Errorf("Minify(%q, %+v) = %q, %v; want %q, %s", tt.css, tt.opts, got.CSS, got.Map, tt.want, tt.wantMap)
		}
	}
}

func TestMinifyBlock(t *testing.T) {
	tests := []struct {
		css     string
		opts    csso.Options
		want    string
		wantErr string
	}{
		{`color:red;color:red`, csso.Options{}, `color:red`, ""},
		{`margin-top:0;margin:1px`, csso.Options{}, `margin:1px`, ""},
		{`foo;color:red`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading '0')`},
		{`foo;color:red`, csso.Options{Restructure: new(false)}, `foo;color:red`, ""},
		{`color:rgba(0,0,0,0);background:none`, csso.Options{}, `color:transparent;background:0 0`, ""},
		{`color:red`, csso.Options{Usage: &csso.Usage{Tags: []string{`a`}}}, `color:red`, ""},
		{`a{b:c}`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading '0')`},
	}
	for _, tt := range tests {
		got, err := csso.MinifyBlock(tt.css, tt.opts)
		if gotErr := errorString(t, err); got.CSS != tt.want || gotErr != tt.wantErr {
			t.Errorf("MinifyBlock(%q, %+v) = %q, %q; want %q, %q", tt.css, tt.opts, got.CSS, gotErr, tt.want, tt.wantErr)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		css      string
		opts     csso.ParseOptions
		wantJSON string
		wantCSS  string
		wantErr  string
	}{
		{`a{color:red!important}`, csso.ParseOptions{}, `{"type":"StyleSheet","loc":null,"children":[{"type":"Rule","loc":null,"prelude":{"type":"SelectorList","loc":null,"children":[{"type":"Selector","loc":null,"children":[{"type":"TypeSelector","loc":null,"name":"a"}]}]},"block":{"type":"Block","loc":null,"children":[{"type":"Declaration","loc":null,"important":true,"property":"color","value":{"type":"Value","loc":null,"children":[{"type":"Identifier","loc":null,"name":"red"}]}}]}}]}`, `a{color:red!important}`, ""},
		{`a{color:red}`, csso.ParseOptions{Positions: true}, `{"type":"StyleSheet","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":12,"line":1,"column":13}},"children":[{"type":"Rule","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":12,"line":1,"column":13}},"prelude":{"type":"SelectorList","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":1,"line":1,"column":2}},"children":[{"type":"Selector","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":1,"line":1,"column":2}},"children":[{"type":"TypeSelector","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":1,"line":1,"column":2}},"name":"a"}]}]},"block":{"type":"Block","loc":{"source":"<unknown>","start":{"offset":1,"line":1,"column":2},"end":{"offset":12,"line":1,"column":13}},"children":[{"type":"Declaration","loc":{"source":"<unknown>","start":{"offset":2,"line":1,"column":3},"end":{"offset":11,"line":1,"column":12}},"important":false,"property":"color","value":{"type":"Value","loc":{"source":"<unknown>","start":{"offset":8,"line":1,"column":9},"end":{"offset":11,"line":1,"column":12}},"children":[{"type":"Identifier","loc":{"source":"<unknown>","start":{"offset":8,"line":1,"column":9},"end":{"offset":11,"line":1,"column":12}},"name":"red"}]}}]}}]}`, `a{color:red}`, ""},
		{`@media screen and (min-width:100px){a[x=y i]:nth-child(2n+1 of .b)::before{x:1/2}}`, csso.ParseOptions{Positions: true, Filename: `f.css`}, `{"type":"StyleSheet","loc":{"source":"f.css","start":{"offset":0,"line":1,"column":1},"end":{"offset":82,"line":1,"column":83}},"children":[{"type":"Atrule","loc":{"source":"f.css","start":{"offset":0,"line":1,"column":1},"end":{"offset":82,"line":1,"column":83}},"name":"media","prelude":{"type":"AtrulePrelude","loc":{"source":"f.css","start":{"offset":7,"line":1,"column":8},"end":{"offset":35,"line":1,"column":36}},"children":[{"type":"MediaQueryList","loc":{"source":"f.css","start":{"offset":7,"line":1,"column":8},"end":{"offset":35,"line":1,"column":36}},"children":[{"type":"MediaQuery","loc":{"source":"f.css","start":{"offset":7,"line":1,"column":8},"end":{"offset":35,"line":1,"column":36}},"children":[{"type":"Identifier","loc":{"source":"f.css","start":{"offset":7,"line":1,"column":8},"end":{"offset":13,"line":1,"column":14}},"name":"screen"},{"type":"Identifier","loc":{"source":"f.css","start":{"offset":14,"line":1,"column":15},"end":{"offset":17,"line":1,"column":18}},"name":"and"},{"type":"MediaFeature","loc":{"source":"f.css","start":{"offset":18,"line":1,"column":19},"end":{"offset":35,"line":1,"column":36}},"name":"min-width","value":{"type":"Dimension","loc":{"source":"f.css","start":{"offset":29,"line":1,"column":30},"end":{"offset":34,"line":1,"column":35}},"value":"100","unit":"px"}}]}]}]},"block":{"type":"Block","loc":{"source":"f.css","start":{"offset":35,"line":1,"column":36},"end":{"offset":82,"line":1,"column":83}},"children":[{"type":"Rule","loc":{"source":"f.css","start":{"offset":36,"line":1,"column":37},"end":{"offset":81,"line":1,"column":82}},"prelude":{"type":"SelectorList","loc":{"source":"f.css","start":{"offset":36,"line":1,"column":37},"end":{"offset":74,"line":1,"column":75}},"children":[{"type":"Selector","loc":{"source":"f.css","start":{"offset":36,"line":1,"column":37},"end":{"offset":74,"line":1,"column":75}},"children":[{"type":"TypeSelector","loc":{"source":"f.css","start":{"offset":36,"line":1,"column":37},"end":{"offset":37,"line":1,"column":38}},"name":"a"},{"type":"AttributeSelector","loc":{"source":"f.css","start":{"offset":37,"line":1,"column":38},"end":{"offset":44,"line":1,"column":45}},"name":{"type":"Identifier","loc":{"source":"f.css","start":{"offset":38,"line":1,"column":39},"end":{"offset":39,"line":1,"column":40}},"name":"x"},"matcher":"=","value":{"type":"Identifier","loc":{"source":"f.css","start":{"offset":40,"line":1,"column":41},"end":{"offset":41,"line":1,"column":42}},"name":"y"},"flags":"i"},{"type":"PseudoClassSelector","loc":{"source":"f.css","start":{"offset":44,"line":1,"column":45},"end":{"offset":66,"line":1,"column":67}},"name":"nth-child","children":[{"type":"Nth","loc":{"source":"f.css","start":{"offset":55,"line":1,"column":56},"end":{"offset":65,"line":1,"column":66}},"nth":{"type":"AnPlusB","loc":{"source":"f.css","start":{"offset":55,"line":1,"column":56},"end":{"offset":59,"line":1,"column":60}},"a":"2","b":"1"},"selector":{"type":"SelectorList","loc":{"source":"f.css","start":{"offset":63,"line":1,"column":64},"end":{"offset":65,"line":1,"column":66}},"children":[{"type":"Selector","loc":{"source":"f.css","start":{"offset":63,"line":1,"column":64},"end":{"offset":65,"line":1,"column":66}},"children":[{"type":"ClassSelector","loc":{"source":"f.css","start":{"offset":63,"line":1,"column":64},"end":{"offset":65,"line":1,"column":66}},"name":"b"}]}]}}]},{"type":"PseudoElementSelector","loc":{"source":"f.css","start":{"offset":66,"line":1,"column":67},"end":{"offset":74,"line":1,"column":75}},"name":"before","children":null}]}]},"block":{"type":"Block","loc":{"source":"f.css","start":{"offset":74,"line":1,"column":75},"end":{"offset":81,"line":1,"column":82}},"children":[{"type":"Declaration","loc":{"source":"f.css","start":{"offset":75,"line":1,"column":76},"end":{"offset":80,"line":1,"column":81}},"important":false,"property":"x","value":{"type":"Value","loc":{"source":"f.css","start":{"offset":77,"line":1,"column":78},"end":{"offset":80,"line":1,"column":81}},"children":[{"type":"Number","loc":{"source":"f.css","start":{"offset":77,"line":1,"column":78},"end":{"offset":78,"line":1,"column":79}},"value":"1"},{"type":"Operator","loc":{"source":"f.css","start":{"offset":78,"line":1,"column":79},"end":{"offset":79,"line":1,"column":80}},"value":"/"},{"type":"Number","loc":{"source":"f.css","start":{"offset":79,"line":1,"column":80},"end":{"offset":80,"line":1,"column":81}},"value":"2"}]}}]}}]}}]}`, `@media screen and (min-width:100px){a[x=y i]:nth-child(2n+1 of.b)::before{x:1/2}}`, ""},
		{`1px solid #fff`, csso.ParseOptions{Context: `value`}, `{"type":"Value","loc":null,"children":[{"type":"Dimension","loc":null,"value":"1","unit":"px"},{"type":"Identifier","loc":null,"name":"solid"},{"type":"Hash","loc":null,"value":"fff"}]}`, `1px solid #fff`, ""},
		{`a, b > c`, csso.ParseOptions{Context: `selectorList`, Positions: true}, `{"type":"SelectorList","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":8,"line":1,"column":9}},"children":[{"type":"Selector","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":1,"line":1,"column":2}},"children":[{"type":"TypeSelector","loc":{"source":"<unknown>","start":{"offset":0,"line":1,"column":1},"end":{"offset":1,"line":1,"column":2}},"name":"a"}]},{"type":"Selector","loc":{"source":"<unknown>","start":{"offset":3,"line":1,"column":4},"end":{"offset":8,"line":1,"column":9}},"children":[{"type":"TypeSelector","loc":{"source":"<unknown>","start":{"offset":3,"line":1,"column":4},"end":{"offset":4,"line":1,"column":5}},"name":"b"},{"type":"Combinator","loc":{"source":"<unknown>","start":{"offset":5,"line":1,"column":6},"end":{"offset":6,"line":1,"column":7}},"name":">"},{"type":"TypeSelector","loc":{"source":"<unknown>","start":{"offset":7,"line":1,"column":8},"end":{"offset":8,"line":1,"column":9}},"name":"c"}]}]}`, `a,b>c`, ""},
		{`color:red;x:y !ie`, csso.ParseOptions{Context: `declarationList`}, `{"type":"DeclarationList","loc":null,"children":[{"type":"Declaration","loc":null,"important":false,"property":"color","value":{"type":"Value","loc":null,"children":[{"type":"Identifier","loc":null,"name":"red"}]}},{"type":"Declaration","loc":null,"important":"ie","property":"x","value":{"type":"Value","loc":null,"children":[{"type":"Identifier","loc":null,"name":"y"}]}}]}`, `color:red;x:y!ie`, ""},
		{`a{`, csso.ParseOptions{}, `{"type":"StyleSheet","loc":null,"children":[{"type":"Rule","loc":null,"prelude":{"type":"SelectorList","loc":null,"children":[{"type":"Selector","loc":null,"children":[{"type":"TypeSelector","loc":null,"name":"a"}]}]},"block":{"type":"Block","loc":null,"children":[]}}]}`, `a{}`, ""},
		{`{color:red}`, csso.ParseOptions{Context: `block`}, `{"type":"Block","loc":null,"children":[{"type":"Declaration","loc":null,"important":false,"property":"color","value":{"type":"Value","loc":null,"children":[{"type":"Identifier","loc":null,"name":"red"}]}}]}`, `{color:red}`, ""},
		{`a{b:c}`, csso.ParseOptions{Context: `nope`}, "", "", "Error: Unknown context `nope`"},
		{`<!-- a{b:\"\0} -->`, csso.ParseOptions{}, `{"type":"StyleSheet","loc":null,"children":[{"type":"CDO","loc":null},{"type":"Rule","loc":null,"prelude":{"type":"SelectorList","loc":null,"children":[{"type":"Selector","loc":null,"children":[{"type":"TypeSelector","loc":null,"name":"a"}]}]},"block":{"type":"Block","loc":null,"children":[{"type":"Declaration","loc":null,"important":false,"property":"b","value":{"type":"Value","loc":null,"children":[{"type":"Identifier","loc":null,"name":"\\\"\\0"}]}}]}},{"type":"CDC","loc":null}]}`, `<!--a{b:\"\0}-->`, ""},
		{`a{color:red}`, csso.ParseOptions{RawValue: true}, `{"type":"StyleSheet","loc":null,"children":[{"type":"Rule","loc":null,"prelude":{"type":"SelectorList","loc":null,"children":[{"type":"Selector","loc":null,"children":[{"type":"TypeSelector","loc":null,"name":"a"}]}]},"block":{"type":"Block","loc":null,"children":[{"type":"Declaration","loc":null,"important":false,"property":"color","value":{"type":"Raw","loc":null,"value":"red"}}]}}]}`, `a{color:red}`, ""},
	}
	for _, tt := range tests {
		ast, err := csso.Parse(tt.css, tt.opts)
		if gotErr := errorString(t, err); gotErr != tt.wantErr {
			t.Errorf("Parse(%q, %+v) error = %q; want %q", tt.css, tt.opts, gotErr, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if got := ast.JSON(); got != tt.wantJSON {
			t.Errorf("Parse(%q, %+v).JSON() =\n%s\nwant\n%s", tt.css, tt.opts, got, tt.wantJSON)
		}
		if got := ast.Generate(); got != tt.wantCSS {
			t.Errorf("Parse(%q, %+v).Generate() = %q; want %q", tt.css, tt.opts, got, tt.wantCSS)
		}
	}
}

func TestParseOnParseError(t *testing.T) {
	tests := []struct {
		css        string
		want       string
		wantErrors []string
	}{
		{"a{color:red;;}b{", "a{color:red;;}b{}", nil},
		{"a{b:c;1x:y}", "a{b:c;1x:y}", []string{"SyntaxError: Identifier is expected"}},
		{"a{color:red}b{x:(}", "a{color:red}b{x:(}}", []string{`SyntaxError: ")" is expected`}},
		{"@media (x{a{b:c}}", "@media (x{a{b:c}}", []string{"SyntaxError: Colon is expected"}},
	}
	for _, tt := range tests {
		var reported []string
		ast, err := csso.Parse(tt.css, csso.ParseOptions{
			OnParseError: func(err *csso.Error) { reported = append(reported, err.Error()) },
		})
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.css, err)
		}
		if got := ast.Generate(); got != tt.want || fmt.Sprint(reported) != fmt.Sprint(tt.wantErrors) {
			t.Errorf("Parse(%q) = %q reporting %q; want %q reporting %q", tt.css, got, reported, tt.want, tt.wantErrors)
		}
	}
}

func TestCompress(t *testing.T) {
	tests := []struct {
		css     string
		context string
		opts    csso.Options
		want    string
		wantErr string
	}{
		{`color:red;color:red`, `declarationList`, csso.Options{}, `color:red`, ""},
		{`red`, `value`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading '0')`},
		{`red`, `value`, csso.Options{Restructure: new(false)}, `red`, ""},
		{`1px solid #ffffff`, `value`, csso.Options{Restructure: new(false)}, `1px solid #fff`, ""},
		{``, `value`, csso.Options{}, ``, ""},
		{`*.a, b`, `selectorList`, csso.Options{Restructure: new(false)}, `.a,b`, ""},
		{`a, b`, `selectorList`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading '0')`},
		{`{color:red;color:blue}`, `block`, csso.Options{}, `{color:#00f}`, ""},
		{`a{color:red}`, `rule`, csso.Options{}, `a{color:red}`, ""},
		{`color:red`, `declaration`, csso.Options{}, `color:red`, ""},
		{`--x:  1  `, `declaration`, csso.Options{}, `--x:1`, ""},
		{`@media print{}`, `atrule`, csso.Options{}, "", `TypeError: Cannot read properties of undefined (reading 'remove')`},
		{`screen and (min-width:0px)`, `mediaQueryList`, csso.Options{Restructure: new(false)}, `screen and (min-width:0px)`, ""},
	}
	for _, tt := range tests {
		ast, err := csso.Parse(tt.css, csso.ParseOptions{Context: tt.context})
		if err != nil {
			t.Fatalf("Parse(%q, %s): %v", tt.css, tt.context, err)
		}
		out, err := csso.Compress(ast, tt.opts)
		got := ""
		if err == nil {
			got = out.Generate()
		}
		if gotErr := errorString(t, err); got != tt.want || gotErr != tt.wantErr {
			t.Errorf("Compress(%q as %s, %+v) = %q, %q; want %q, %q", tt.css, tt.context, tt.opts, got, gotErr, tt.want, tt.wantErr)
		}
	}
}

func TestCompressClone(t *testing.T) {
	ast, err := csso.Parse(".a{color:#ff0000}.b{color:red}")
	if err != nil {
		t.Fatal(err)
	}
	out, err := csso.Compress(ast, csso.Options{Clone: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Generate(); got != ".a,.b{color:red}" {
		t.Errorf("compressed = %q", got)
	}
	if got := ast.Generate(); got != ".a{color:#ff0000}.b{color:red}" {
		t.Errorf("original changed to %q", got)
	}
}

func TestCompressNil(t *testing.T) {
	out, err := csso.Compress(nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Type() != "StyleSheet" || out.Generate() != "" {
		t.Errorf("Compress(nil) = %s %q", out.Type(), out.Generate())
	}
}

func TestLogger(t *testing.T) {
	var titles []string
	_, err := csso.Minify("a{x:1}", csso.Options{Logger: func(title string, ast *csso.AST) {
		if ast != nil {
			title += " " + ast.Generate()
		}
		titles = append(titles, title)
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Compress block #1", "init a{x:1}", "clean a{x:1}", "replace a{x:1}",
		"prepare a{x:1}", "mergeAtrule a{x:1}", "initialMergeRuleset a{x:1}", "disjoinRuleset a{x:1}",
		"restructShorthand a{x:1}", "restructBlock a{x:1}", "mergeRuleset a{x:1}", "restructRuleset a{x:1}",
	}
	if fmt.Sprint(titles) != fmt.Sprint(want) {
		t.Errorf("logged %q\nwant %q", titles, want)
	}
}

func TestHandlers(t *testing.T) {
	var seen []string
	record := func(stage string) func(*csso.AST, csso.Options) error {
		return func(ast *csso.AST, _ csso.Options) error {
			seen = append(seen, stage+" "+ast.Generate())
			return nil
		}
	}
	r, err := csso.Minify("a { color: #f00 }", csso.Options{
		BeforeCompress: []func(*csso.AST, csso.Options) error{record("before")},
		AfterCompress:  []func(*csso.AST, csso.Options) error{record("after")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "[before a{color:#f00} after a{color:red}]"; fmt.Sprint(seen) != want || r.CSS != "a{color:red}" {
		t.Errorf("handlers saw %q, result %q", seen, r.CSS)
	}

	stop := errors.New("stop")
	_, err = csso.Minify("a{}", csso.Options{
		BeforeCompress: []func(*csso.AST, csso.Options) error{func(*csso.AST, csso.Options) error { return stop }},
	})
	if !errors.Is(err, stop) {
		t.Errorf("handler error = %v", err)
	}
}

func ExampleMinify() {
	result, err := csso.Minify(".a { color: #ff0000; } .b { color: red }")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.CSS)
	// Output: .a,.b{color:red}
}

func ExampleMinifyBlock() {
	result, err := csso.MinifyBlock("color: rgba(0, 0, 0, 0); margin: 0px 0px 0px 0px")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.CSS)
	// Output: color:transparent;margin:0
}
