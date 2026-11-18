package lexer

import "github.com/css/csso-go/internal/csstree"

// prepareTokens generates value into the token list the matcher consumes,
// each token attributed to the node that emitted it (css-tree's
// prepare-tokens.js). A nil value yields no tokens, like an empty string.
//
// csso imports css-tree's default lexer, whose generator serializes strings
// and URLs with css-tree's own encoders (double-quoted strings, unquoted
// url()) rather than csso's shortest-form overrides, so those token values
// are re-encoded here.
func prepareTokens(value csstree.Node) []token {
	if value == nil {
		return nil
	}

	generated := csstree.GenerateTokens(value)
	tokens := make([]token, len(generated))
	for i, g := range generated {
		tok := token{typ: g.Type, value: g.Value, node: g.Node}
		switch n := g.Node.(type) {
		case *csstree.String:
			if g.Type == csstree.StringToken {
				tok.value = csstree.EncodeString(n.Value, false)
			}
		case *csstree.Url:
			if g.Type == csstree.URLToken {
				tok.value = csstree.EncodeURL(n.Value)
			}
		}
		tokens[i] = tok
	}
	return tokens
}
