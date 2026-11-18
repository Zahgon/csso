// Package lexer is a port of css-tree 2.2.0's lexer: it matches CSS values
// (as AST) against the value definition syntaxes of mdn-data 2.0.28, patched
// by css-tree, and answers which syntax each value node was matched as. csso
// relies on it to decide whether an identifier is a color or whether a zero
// percentage may become a length.
package lexer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/css/csso-go/internal/csstree"
	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// Descriptor is a named syntax known to a Lexer.
type Descriptor struct {
	// Kind is "Type", "Property", "AtrulePrelude" or "AtruleDescriptor".
	Kind string
	// Name is the type, property, at-rule or descriptor name.
	Name string
	// Parent is the at-rule of an at-rule descriptor.
	Parent string
	// Syntax is the parsed definition; nil for generic types, which are
	// matched by code.
	Syntax defsyntax.Node

	match *state
}

// Atrule holds the syntaxes of an at-rule.
type Atrule struct {
	Name string
	// Prelude is nil when the at-rule takes no prelude.
	Prelude *Descriptor
	// Descriptors is nil when the at-rule has no descriptor dictionary.
	Descriptors map[string]*Descriptor
}

// Lexer matches values against the syntaxes of a set of definitions.
// A Lexer is immutable once built and safe for concurrent use.
type Lexer struct {
	types           map[string]*Descriptor
	properties      map[string]*Descriptor
	atrules         map[string]*Atrule
	cssWideKeywords *state
}

// New builds a lexer from definitions, adding css-tree's generic types (which
// replace definitions of the same name).
func New(defs *Definitions) (*Lexer, error) {
	wide, err := defsyntax.Parse(strings.Join(cssWideKeywords, " | "))
	if err != nil {
		return nil, fmt.Errorf("lexer: CSS-wide keywords: %w", err)
	}

	l := &Lexer{
		types:           make(map[string]*Descriptor, len(defs.Types)+len(genericTypes)),
		properties:      make(map[string]*Descriptor, len(defs.Properties)),
		atrules:         make(map[string]*Atrule, len(defs.Atrules)),
		cssWideKeywords: buildMatchGraph(wide, nil),
	}

	for name, syntax := range defs.Types {
		if l.types[name], err = newDescriptor(syntax, "Type", name, ""); err != nil {
			return nil, err
		}
	}
	for name, fn := range genericTypes {
		ref := &defsyntax.Type{Name: name}
		l.types[name] = &Descriptor{Kind: "Type", Name: name, match: buildGenericMatchGraph(fn, ref)}
	}

	for name, def := range defs.Atrules {
		atrule := &Atrule{Name: name}
		if def.Prelude != "" {
			if atrule.Prelude, err = newDescriptor(def.Prelude, "AtrulePrelude", name, ""); err != nil {
				return nil, err
			}
		}
		if def.Descriptors != nil {
			atrule.Descriptors = make(map[string]*Descriptor, len(def.Descriptors))
			for descName, syntax := range def.Descriptors {
				if atrule.Descriptors[descName], err = newDescriptor(syntax, "AtruleDescriptor", descName, name); err != nil {
					return nil, err
				}
			}
		}
		l.atrules[name] = atrule
	}

	for name, syntax := range defs.Properties {
		if l.properties[name], err = newDescriptor(syntax, "Property", name, ""); err != nil {
			return nil, err
		}
	}

	return l, nil
}

// newDescriptor parses a definition and builds its match graph. css-tree does
// both lazily on first use; doing it upfront reports broken definitions
// early and keeps the Lexer free of mutation.
func newDescriptor(source, kind, name, parent string) (*Descriptor, error) {
	syntax, err := defsyntax.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("lexer: %s %s: %w", kind, name, err)
	}

	// the reference recorded as the root of match trees; only types and
	// properties show up in traces
	var ref defsyntax.Node
	switch kind {
	case "Type":
		ref = &defsyntax.Type{Name: name}
	case "Property":
		ref = &defsyntax.Property{Name: name}
	}

	return &Descriptor{
		Kind:   kind,
		Name:   name,
		Parent: parent,
		Syntax: syntax,
		match:  buildMatchGraph(syntax, ref),
	}, nil
}

// Default returns the lexer css-tree 2.2.0 exports: mdn-data 2.0.28 with
// css-tree's patch and generic types. It is built on first use.
var Default = sync.OnceValue(func() *Lexer {
	defs, err := MDNDefinitions()
	if err != nil {
		panic(err)
	}
	l, err := New(defs)
	if err != nil {
		panic(err)
	}
	return l
})

// Type returns the descriptor of a type, or nil.
func (l *Lexer) Type(name string) *Descriptor {
	return l.types[name]
}

// Property returns the descriptor of a property, or nil. A vendor-prefixed
// name without a definition of its own falls back to the unprefixed property,
// and hacks such as `*color` are ignored.
func (l *Lexer) Property(name string) *Descriptor {
	p := csstree.Property(name)
	if d := l.properties[p.Name]; d != nil || p.Vendor == "" {
		return d
	}
	return l.properties[p.Basename]
}

// Atrule returns the syntaxes of an at-rule, or nil. A vendor-prefixed name
// without a definition of its own falls back to the unprefixed at-rule.
func (l *Lexer) Atrule(name string) *Atrule {
	k := csstree.Keyword(name)
	if a := l.atrules[k.Name]; a != nil || k.Vendor == "" {
		return a
	}
	return l.atrules[k.Basename]
}

// MatchDeclaration matches the value of a declaration against its property's
// syntax.
func (l *Lexer) MatchDeclaration(decl *csstree.Declaration) *MatchResult {
	return l.MatchProperty(decl.Property, decl.Value)
}

// MatchProperty matches value against the syntax of property, also accepting
// the CSS-wide keywords. Custom properties are not matched.
func (l *Lexer) MatchProperty(property string, value csstree.Node) *MatchResult {
	descriptor, err := l.matchableProperty(property)
	if err != nil {
		return failedMatch(err)
	}
	return l.matchSyntax(descriptor, prepareTokens(value), value, true)
}

// matchableProperty returns the descriptor MatchProperty matches against, or
// the reason it does not match at all.
func (l *Lexer) matchableProperty(property string) (*Descriptor, error) {
	if csstree.Property(property).Custom {
		return nil, ErrCustomProperty
	}
	descriptor := l.Property(property)
	if descriptor == nil {
		return nil, &SyntaxReferenceError{Kind: "Unknown property", Reference: property}
	}
	return descriptor, nil
}

// MatchType matches value against the syntax of a type.
func (l *Lexer) MatchType(typeName string, value csstree.Node) *MatchResult {
	descriptor := l.Type(typeName)
	if descriptor == nil {
		return failedMatch(&SyntaxReferenceError{Kind: "Unknown type", Reference: typeName})
	}
	return l.matchSyntax(descriptor, prepareTokens(value), value, false)
}

// valueHasVar reports whether the value contains a var() function.
func valueHasVar(tokens []token) bool {
	for _, tok := range tokens {
		if toLowerCase(tok.value) == "var(" {
			return true
		}
	}
	return false
}

// matchSyntax matches the tokens of value against a descriptor, trying the
// CSS-wide keywords first when asked to.
func (l *Lexer) matchSyntax(descriptor *Descriptor, tokens []token, value csstree.Node, useCSSWideKeywords bool) *MatchResult {
	if valueHasVar(tokens) {
		return failedMatch(ErrVarNotSupported)
	}

	if useCSSWideKeywords {
		outcome, tree, err := matchAsTree(tokens, l.cssWideKeywords, l)
		if err != nil {
			return failedMatch(err)
		}
		if tree != nil {
			return &MatchResult{Matched: tree, Iterations: outcome.iterations}
		}
	}

	outcome, tree, err := matchAsTree(tokens, descriptor.match, l)
	if err != nil {
		return failedMatch(err)
	}
	if tree == nil {
		return &MatchResult{
			Iterations: outcome.iterations,
			Error:      newSyntaxMatchError(outcome, descriptor.Syntax, value),
		}
	}
	return &MatchResult{Matched: tree, Iterations: outcome.iterations}
}
