package nodes

import (
	"testing"

	"github.com/css/csso-go/internal/csstree"
)

func TestHasNoChildren(t *testing.T) {
	tests := []struct {
		node csstree.Node
		want bool
	}{
		{nil, true},
		{&csstree.Raw{Value: "x"}, true},
		{&csstree.Value{Children: csstree.NewList()}, true},
		{&csstree.Value{Children: csstree.NewListOf(&csstree.Identifier{Name: "x"})}, false},
		{Block(nil), true},
	}
	for _, tt := range tests {
		if got := HasNoChildren(tt.node); got != tt.want {
			t.Errorf("HasNoChildren(%#v) = %v; want %v", tt.node, got, tt.want)
		}
	}
}

func TestChildList(t *testing.T) {
	if _, err := ChildList(&csstree.Raw{}, "forEach"); err == nil ||
		err.Error() != "TypeError: Cannot read properties of undefined (reading 'forEach')" {
		t.Errorf("ChildList(Raw) error = %v", err)
	}
	if _, err := ChildList(&csstree.PseudoClassSelector{Name: "not"}, "first"); err == nil ||
		err.Error() != "TypeError: Cannot read properties of null (reading 'first')" {
		t.Errorf("ChildList(:not) error = %v", err)
	}
	list := csstree.NewList()
	if got, err := ChildList(&csstree.Selector{Children: list}, "forEach"); got != list || err != nil {
		t.Errorf("ChildList(Selector) = %v, %v", got, err)
	}
}

func TestFunctionName(t *testing.T) {
	if got := FunctionName(&csstree.Function{Name: "calc"}); got != "calc" {
		t.Errorf("FunctionName(calc()) = %q", got)
	}
	if got := FunctionName(&csstree.PseudoClassSelector{Name: "not"}); got != "not" {
		t.Errorf("FunctionName(:not) = %q", got)
	}
	if got := FunctionName(&csstree.Identifier{Name: "x"}); got != "" {
		t.Errorf("FunctionName(ident) = %q", got)
	}
}
