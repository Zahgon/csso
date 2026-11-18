package lexer

import (
	"errors"
	"regexp"
	"strings"

	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

var (
	// ErrVarNotSupported is reported for values containing var(), which
	// css-tree does not match.
	ErrVarNotSupported = errors.New("Matching for a tree with var() is not supported")
	// ErrCustomProperty is reported when matching a custom property, whose
	// value css-tree does not match.
	ErrCustomProperty = errors.New("Lexer matching doesn't applicable for custom properties")
)

// SyntaxReferenceError reports an unknown property, type or at-rule.
type SyntaxReferenceError struct {
	// Kind describes what is unknown, e.g. "Unknown property".
	Kind string
	// Reference is the unknown name.
	Reference string
}

// Error returns the message css-tree gives, e.g. "Unknown property `foo`".
func (e *SyntaxReferenceError) Error() string {
	if e.Reference == "" {
		return e.Kind
	}
	return e.Kind + " `" + e.Reference + "`"
}

// SyntaxMatchError reports a value that does not match a syntax, locating the
// first token the matcher could not get past.
type SyntaxMatchError struct {
	// Reason is the matcher's exit reason, e.g. "Mismatch".
	Reason string
	// Syntax is the generated syntax matched against, "<generic>" for a
	// generic type.
	Syntax string
	// CSS is the value as the concatenation of its tokens.
	CSS string
	// MismatchOffset and MismatchLength locate the offending token in CSS,
	// in UTF-16 code units.
	MismatchOffset int
	MismatchLength int
	// Loc is the source range of the mismatch.
	Loc csstree.Location
}

// Error returns css-tree's message: the reason, the syntax, the value and a
// caret under the mismatch.
func (e *SyntaxMatchError) Error() string {
	css := e.CSS
	if css == "" {
		css = "<empty string>"
	}
	return e.Reason + "\n" +
		"  syntax: " + e.Syntax + "\n" +
		"   value: " + css + "\n" +
		"  --------" + strings.Repeat("-", e.MismatchOffset) + "^"
}

func newSyntaxMatchError(outcome *matchOutcome, syntax defsyntax.Node, value csstree.Node) *SyntaxMatchError {
	e := &SyntaxMatchError{Reason: outcome.reason, Syntax: "<generic>"}
	if syntax != nil {
		e.Syntax = defsyntax.Generate(syntax, defsyntax.GenerateOptions{})
	}

	var start, end csstree.Position
	e.CSS, e.MismatchOffset, e.MismatchLength, start, end = locateMismatch(outcome, value)

	e.Loc = csstree.Location{Source: "<unknown>", Start: start, End: end}
	if loc := nodeLoc(value); loc != nil && loc.Source != "" {
		e.Loc.Source = loc.Source
	}
	return e
}

func nodeLoc(node csstree.Node) *csstree.Location {
	if node == nil {
		return nil
	}
	return node.Loc()
}

// fromLoc returns the start or end position of node, if it has a location.
func fromLoc(node csstree.Node, end bool) (csstree.Position, bool) {
	loc := nodeLoc(node)
	if loc == nil {
		return csstree.Position{}, false
	}
	if end {
		return loc.End, true
	}
	return loc.Start, true
}

var lineBreak = regexp.MustCompile(`\n|\r\n?|\f`)

// advance moves pos over the text extra, as css-tree's buildLoc does.
func advance(pos csstree.Position, extra string) csstree.Position {
	if extra == "" {
		return pos
	}
	lines := lineBreak.Split(extra, -1)
	pos.Offset += csstree.UTF16Len(extra)
	pos.Line += len(lines) - 1
	if len(lines) == 1 {
		pos.Column += csstree.UTF16Len(extra)
	} else {
		pos.Column = csstree.UTF16Len(lines[len(lines)-1]) + 1
	}
	return pos
}

// locateMismatch finds the token at the longest match in the generated value
// and the source range it corresponds to.
func locateMismatch(outcome *matchOutcome, node csstree.Node) (css string, mismatchOffset, mismatchLength int, start, end csstree.Position) {
	tokens := outcome.tokens
	longestMatch := outcome.longestMatch

	var badNode csstree.Node
	if longestMatch < len(tokens) && tokens[longestMatch].node != node {
		badNode = tokens[longestMatch].node
	}

	var b strings.Builder
	mismatchByteOffset, mismatchByteLength := 0, 0
	entries := 0
	for i, tok := range tokens {
		if i == longestMatch {
			mismatchLength = csstree.UTF16Len(tok.value)
			mismatchOffset = csstree.UTF16Len(b.String())
			mismatchByteOffset, mismatchByteLength = b.Len(), len(tok.value)
		}
		if badNode != nil && tok.node == badNode {
			if i <= longestMatch {
				entries++
			} else {
				entries = 0
			}
		}
		b.WriteString(tok.value)
	}
	css = b.String()

	defaultLoc := csstree.Position{Offset: 0, Line: 1, Column: 1}

	if longestMatch == len(tokens) || entries > 1 {
		target := badNode
		if target == nil {
			target = node
		}
		var ok bool
		if start, ok = fromLoc(target, true); !ok {
			start = advance(defaultLoc, css)
		}
		return css, mismatchOffset, mismatchLength, start, start
	}

	var ok bool
	if start, ok = fromLoc(badNode, false); !ok {
		base, hasBase := fromLoc(node, false)
		if !hasBase {
			base = defaultLoc
		}
		start = advance(base, css[:mismatchByteOffset])
	}
	if end, ok = fromLoc(badNode, true); !ok {
		end = advance(start, css[mismatchByteOffset:mismatchByteOffset+mismatchByteLength])
	}
	return css, mismatchOffset, mismatchLength, start, end
}
