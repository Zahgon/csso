package usage

import "testing"

func TestNewList(t *testing.T) {
	if l := NewList(nil, nil, nil); l != nil {
		t.Errorf("NewList(nil, nil, nil) = %+v; want nil", l)
	}

	l := NewList([]string{"DIV", "a"}, nil, []string{})
	if l == nil {
		t.Fatal("NewList returned nil")
	}
	if !l.Tags["div"] || !l.Tags["a"] || l.Tags["DIV"] {
		t.Errorf("tags %v are not lowercased", l.Tags)
	}
	if l.IDs != nil {
		t.Errorf("ids = %v; want nil (no restriction)", l.IDs)
	}
	if l.Classes == nil || len(l.Classes) != 0 {
		t.Errorf("classes = %v; want an empty set", l.Classes)
	}
}

func TestNewScopes(t *testing.T) {
	scopes, err := NewScopes([][]string{{"a", "b"}, {"c"}})
	if err != nil {
		t.Fatal(err)
	}
	if scopes["a"] != 1 || scopes["b"] != 1 || scopes["c"] != 2 || scopes["d"] != 0 {
		t.Errorf("scopes = %v", scopes)
	}

	if scopes, err := NewScopes(nil); scopes != nil || err != nil {
		t.Errorf("NewScopes(nil) = %v, %v", scopes, err)
	}

	// error messages recorded from csso 5.0.5
	for _, tt := range []struct {
		scopes [][]string
		want   string
	}{
		{[][]string{{"a"}, nil}, "Error: Wrong usage format"},
		{[][]string{{"a"}, {"a"}}, "Error: Class can't be used for several scopes: a"},
	} {
		if _, err := NewScopes(tt.scopes); err == nil || err.Error() != tt.want {
			t.Errorf("NewScopes(%q) error = %v; want %s", tt.scopes, err, tt.want)
		}
	}
}
