package replace

import (
	"testing"

	"github.com/css/csso-go/internal/csstree"
)

// The expectations below were recorded from csso 5.0.5 on Node.js 24 by
// running csso's replace pass alone on the parsed stylesheet.

func TestRun(t *testing.T) {
	tests := []struct {
		css     string
		want    string
		wantErr string
	}{
		{`a{width:0.50px;height:+.0em;margin:00.10% 0px 0% 0vh;flex:0px;line-height:1.0}`, `a{width:.5px;height:0;margin:.1%0 0 0;flex:0px;line-height:1}`, ""},
		{`@media (min-width:0px){a{x:0px}}`, `@media (min-width:0px){a{x:0}}`, ""},
		{`a{width:calc(0px + 0%);padding:0%}`, `a{width:calc(0px + 0%);padding:0}`, ""},
		{`a{color:RED;background:white;border-color:grey;fill:darkslategray}`, `a{color:red;background:#fff;border-color:gray;fill:#2f4f4f}`, ""},
		{`a{color:rgb(255,0,0);x:rgba(0,0,0,0);y:rgba(255,255,255,1);z:hsl(120,100%,25%);w:hsla(0,0%,100%,.25)}`, `a{color:red;x:transparent;y:#fff;z:green;w:rgba(255,255,255,.25)}`, ""},
		{`a{background-image:linear-gradient(rgba(1,2,3,0),red)}`, `a{background-image:linear-gradient(rgba(1,2,3,0),red)}`, ""},
		{`a{color:#FFFFFF;b:#F00;c:#ff000080}`, `a{color:#fff;b:red;c:#ff000080}`, ""},
		{`a[x="b"],a[x="1"],a[x=""],a[x='--y'],a[x="-"]{y:z}`, `a[x=b],a[x="1"],a[x=""],a[x="--y"],a[x="-"]{y:z}`, ""},
		{`a{background:url(a\b.png)}`, `a{background:url(a\b.png)}`, ""},
		{`@keyframes k{from{x:1}100%{x:2}100.0%{x:3}}`, `@keyframes k{0%{x:1}to{x:2}to{x:3}}`, ""},
		{`a{font:normal normal 12px/normal x;font-weight:bold;background:none;border:none}`, `a{font:12px x;font-weight:700;background:0 0;border:0}`, ""},
		{`a{font:normal}`, `a{font:normal}`, ""},
		{"@keyframes k{@foo bar;}", "", "TypeError: Cannot read properties of undefined (reading 'forEach')"},
	}
	for _, tt := range tests {
		ast, err := csstree.Parse(tt.css, nil)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.css, err)
		}
		err = Run(ast)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if got := csstree.Generate(ast); (err == nil && got != tt.want) || gotErr != tt.wantErr {
			t.Errorf("Run(%q) = %q, %q; want %q, %q", tt.css, got, gotErr, tt.want, tt.wantErr)
		}
	}
}
