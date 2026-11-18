package csstree

import "testing"

// TestGenerateTokensStringURL checks the contract the lexer relies on: a
// String or Url node yields exactly one token attributed to it.
func TestGenerateTokensStringURL(t *testing.T) {
	ast, err := Parse(`a{b:x"a'b" url( "c d" ) url(e)'f'}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[Node]int{}
	for _, tok := range GenerateTokens(ast) {
		switch tok.Node.(type) {
		case *String, *Url:
			counts[tok.Node]++
		}
	}
	if len(counts) != 4 {
		t.Fatalf("got tokens for %d String/Url nodes, want 4", len(counts))
	}
	for node, n := range counts {
		if n != 1 {
			t.Errorf("%s node emitted %d tokens, want 1", node.Type(), n)
		}
	}
}
