package lexer

import (
	"compress/gzip"
	"encoding/json"
	"maps"
	"os"
	"testing"
)

// TestMDNDefinitions compares the patched dictionary with css-tree's data.js
// export, recorded under Node 24 by testdata/gen-fixtures.mjs.
func TestMDNDefinitions(t *testing.T) {
	f, err := os.Open("testdata/definitions.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}

	var want struct {
		Types      map[string]string `json:"types"`
		Properties map[string]string `json:"properties"`
		Atrules    map[string]struct {
			Prelude     *string           `json:"prelude"`
			Descriptors map[string]string `json:"descriptors"`
		} `json:"atrules"`
	}
	if err := json.NewDecoder(zr).Decode(&want); err != nil {
		t.Fatal(err)
	}

	got, err := MDNDefinitions()
	if err != nil {
		t.Fatal(err)
	}

	if !maps.Equal(got.Types, want.Types) {
		t.Errorf("types differ from css-tree's (%d vs %d entries)", len(got.Types), len(want.Types))
		for name, syntax := range want.Types {
			if got.Types[name] != syntax {
				t.Errorf("type %s = %q, want %q", name, got.Types[name], syntax)
			}
		}
	}
	if !maps.Equal(got.Properties, want.Properties) {
		t.Errorf("properties differ from css-tree's (%d vs %d entries)", len(got.Properties), len(want.Properties))
		for name, syntax := range want.Properties {
			if got.Properties[name] != syntax {
				t.Errorf("property %s = %q, want %q", name, got.Properties[name], syntax)
			}
		}
	}

	if len(got.Atrules) != len(want.Atrules) {
		t.Errorf("%d at-rules, want %d", len(got.Atrules), len(want.Atrules))
	}
	for name, w := range want.Atrules {
		g, ok := got.Atrules[name]
		if !ok {
			t.Errorf("missing at-rule %s", name)
			continue
		}
		wantPrelude := ""
		if w.Prelude != nil {
			wantPrelude = *w.Prelude
		}
		if g.Prelude != wantPrelude {
			t.Errorf("at-rule %s prelude = %q, want %q", name, g.Prelude, wantPrelude)
		}
		if (g.Descriptors == nil) != (w.Descriptors == nil) || !maps.Equal(g.Descriptors, w.Descriptors) {
			t.Errorf("at-rule %s descriptors = %v, want %v", name, g.Descriptors, w.Descriptors)
		}
	}
}

// TestLexerLookups covers the descriptor lookups around matching.
func TestLexerLookups(t *testing.T) {
	l := Default()

	for _, tc := range []struct {
		property string
		want     string
	}{
		{"color", "color"},
		{"COLOR", "color"},
		{"*color", "color"},
		{"-webkit-box-shadow", "box-shadow"},
		{"-moz-osx-font-smoothing", "-moz-osx-font-smoothing"},
		{"-foo-bar", ""},
		{"--custom", ""},
		{"unknown", ""},
	} {
		got := ""
		if d := l.Property(tc.property); d != nil {
			got = d.Name
		}
		if got != tc.want {
			t.Errorf("Property(%q) = %q, want %q", tc.property, got, tc.want)
		}
	}

	if d := l.Type("length"); d == nil || d.Syntax != nil {
		t.Errorf("Type(length) = %+v, want a generic type", d)
	}
	if d := l.Type("color"); d == nil || d.Syntax == nil {
		t.Errorf("Type(color) = %+v, want a defined type", d)
	}

	if a := l.Atrule("-webkit-keyframes"); a == nil || a.Name != "keyframes" || a.Prelude == nil {
		t.Errorf("Atrule(-webkit-keyframes) = %+v", a)
	}
	if a := l.Atrule("font-face"); a == nil || a.Prelude != nil || a.Descriptors["unicode-range"] == nil {
		t.Errorf("Atrule(font-face) = %+v", a)
	}
}
