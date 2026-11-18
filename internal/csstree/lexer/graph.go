package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/css/csso-go/internal/csstree/lexer/defsyntax"
)

// stateKind enumerates the node types of css-tree's match graph (and the two
// transient states the matcher creates for MatchOnce groups).
type stateKind uint8

const (
	stateMatch stateKind = iota
	stateMismatch
	stateDisallowEmpty
	stateIf
	stateEnum
	stateMatchOnce
	stateMatchOnceBuffer
	stateAddMatchOnce
	stateMatchGraph
	stateGeneric
	stateType
	stateProperty
	stateKeyword
	stateAtKeyword
	stateFunction
	stateToken
	stateString
	stateComma
)

// state is a node of a match graph, built from a definition syntax tree by
// buildMatchGraph as css-tree's match-graph.js does.
type state struct {
	kind stateKind

	// If: test match, continue with then on success and with els otherwise.
	// MatchGraph: match is the graph root.
	match, then, els *state

	// Enum: alternatives keyed by lower-cased keyword or function name.
	enum map[string]*state

	// MatchOnce: terms to match in any order, all of them when all is set.
	terms []*state
	all   bool

	// MatchOnceBuffer and AddMatchOnce: the MatchOnce group being matched,
	// the next term to try and the bit mask of the terms matched so far.
	once  *state
	index int
	mask  int

	// Generic: the matcher of a generic type.
	generic genericFunc

	// Type, Property, Keyword, AtKeyword, Function: the name to match.
	name string
	// Token, String: the literal to match.
	value string

	// syntax is the definition node a matched token or opened syntax is
	// attributed to: the Type/Property/Keyword/... node itself, the
	// Multiplier of an implied comma, or for a MatchGraph the reference
	// of the descriptor the graph belongs to (nil when anonymous).
	syntax defsyntax.Node
}

// The terminal states shared by all graphs.
var (
	matchState         = &state{kind: stateMatch}
	mismatchState      = &state{kind: stateMismatch}
	disallowEmptyState = &state{kind: stateDisallowEmpty}
)

func createCondition(match, then, els *state) *state {
	// reduce node count
	if then == matchState && els == mismatchState {
		return match
	}
	if match == matchState && then == matchState && els == matchState {
		return match
	}
	if match.kind == stateIf && match.els == mismatchState && then == matchState {
		then = match.then
		match = match.match
	}
	return &state{kind: stateIf, match: match, then: then, els: els}
}

// isFunctionType reports whether a type name refers to a function, e.g.
// "rgb()".
func isFunctionType(name string) bool {
	return len(name) > 2 && strings.HasSuffix(name, "()")
}

func isEnumCompatible(term *state) bool {
	switch term.kind {
	case stateKeyword, stateAtKeyword, stateFunction:
		return true
	case stateType:
		return isFunctionType(term.name)
	}
	return false
}

func buildGroupMatchGraph(combinator string, terms []*state, atLeastOneTermMatched bool) *state {
	switch combinator {
	case " ":
		// Juxtaposing components means that all of them must occur, in the
		// given order: match a, then b, then c.
		result := matchState
		for i := len(terms) - 1; i >= 0; i-- {
			result = createCondition(terms[i], result, mismatchState)
		}
		return result

	case "|":
		// Exactly one of the alternatives must occur. Runs of keywords and
		// functions are folded into an Enum looked up by name.
		result := mismatchState
		var enum map[string]*state

		for i := len(terms) - 1; i >= 0; i-- {
			term := terms[i]

			if isEnumCompatible(term) {
				if enum == nil && i > 0 && isEnumCompatible(terms[i-1]) {
					enum = map[string]*state{}
					result = createCondition(&state{kind: stateEnum, enum: enum}, matchState, result)
				}

				if enum != nil {
					key := term.name
					if isFunctionType(key) {
						key = key[:len(key)-1]
					}
					key = toLowerCase(key)
					if _, dup := enum[key]; !dup {
						enum[key] = term
						continue
					}
				}
			}

			enum = nil
			result = createCondition(term, matchState, result)
		}
		return result

	case "&&", "||":
		// && requires all components, || at least one, in any order. Large
		// groups use MatchOnce since the combination tree grows as N!.
		all := combinator == "&&"
		if len(terms) > 5 {
			return &state{kind: stateMatchOnce, terms: terms, all: all}
		}

		// match a then [b && c], else match b then [a && c], else ...
		result := mismatchState
		if !all && atLeastOneTermMatched {
			result = matchState
		}
		for i := len(terms) - 1; i >= 0; i-- {
			term := terms[i]
			then := matchState
			if len(terms) > 1 {
				rest := make([]*state, 0, len(terms)-1)
				for _, t := range terms {
					if t != term {
						rest = append(rest, t)
					}
				}
				then = buildGroupMatchGraph(combinator, rest, !all)
			}
			result = createCondition(term, then, result)
		}
		return result
	}

	panic(fmt.Sprintf("lexer: unknown combinator %q", combinator))
}

func buildMultiplierMatchGraph(node *defsyntax.Multiplier) *state {
	result := matchState
	matchTerm := buildMatchGraphInternal(node.Term)

	if node.Max == 0 {
		// disable repeating of an empty match to prevent an infinite loop
		matchTerm = createCondition(matchTerm, disallowEmptyState, mismatchState)

		// the occurrence count is unlimited: make a cycle that collects
		// more terms on each following match
		result = createCondition(matchTerm, nil, mismatchState)
		result.then = createCondition(matchState, matchState, result)
		if node.Comma {
			result.then.els = createCondition(&state{kind: stateComma, syntax: node}, result, mismatchState)
		}
	} else {
		// a chain of optional matches for the [min .. max] interval
		start := max(node.Min, 1)
		for i := start; i <= node.Max; i++ {
			if node.Comma && result != matchState {
				result = createCondition(&state{kind: stateComma, syntax: node}, result, mismatchState)
			}
			result = createCondition(matchTerm, createCondition(matchState, matchState, result), mismatchState)
		}
	}

	if node.Min == 0 {
		// allow zero matches
		result = createCondition(matchState, matchState, result)
	} else {
		// a chain of required matches for [0 .. min - 1]
		for i := 0; i < node.Min-1; i++ {
			if node.Comma && result != matchState {
				result = createCondition(&state{kind: stateComma, syntax: node}, result, mismatchState)
			}
			result = createCondition(matchTerm, result, mismatchState)
		}
	}

	return result
}

func buildMatchGraphInternal(node defsyntax.Node) *state {
	switch n := node.(type) {
	case *defsyntax.Group:
		terms := make([]*state, len(n.Terms))
		for i, term := range n.Terms {
			terms[i] = buildMatchGraphInternal(term)
		}
		result := buildGroupMatchGraph(n.Combinator, terms, false)
		if n.DisallowEmpty {
			result = createCondition(result, disallowEmptyState, mismatchState)
		}
		return result

	case *defsyntax.Multiplier:
		return buildMultiplierMatchGraph(n)

	case *defsyntax.Type:
		return &state{kind: stateType, name: n.Name, syntax: n}

	case *defsyntax.Property:
		return &state{kind: stateProperty, name: n.Name, syntax: n}

	case *defsyntax.Keyword:
		return &state{kind: stateKeyword, name: toLowerCase(n.Name), syntax: n}

	case *defsyntax.AtKeyword:
		return &state{kind: stateAtKeyword, name: "@" + toLowerCase(n.Name), syntax: n}

	case *defsyntax.Function:
		return &state{kind: stateFunction, name: toLowerCase(n.Name) + "(", syntax: n}

	case *defsyntax.String:
		// a one character string is matched as a Token
		inner := n.Value[1 : len(n.Value)-1]
		if utf8.RuneCountInString(n.Value) == 3 {
			return &state{kind: stateToken, value: inner, syntax: n}
		}
		return &state{kind: stateString, value: strings.ReplaceAll(inner, `\'`, "'"), syntax: n}

	case *defsyntax.Token:
		return &state{kind: stateToken, value: n.Value, syntax: n}

	case *defsyntax.Comma:
		return &state{kind: stateComma, syntax: n}
	}

	panic(fmt.Sprintf("lexer: unknown definition node type %T", node))
}

// buildMatchGraph builds the match graph of a definition syntax tree. ref is
// the descriptor reference recorded as the syntax of the match tree root.
func buildMatchGraph(syntax defsyntax.Node, ref defsyntax.Node) *state {
	return &state{kind: stateMatchGraph, match: buildMatchGraphInternal(syntax), syntax: ref}
}

// buildGenericMatchGraph wraps the matcher of a generic type into a graph.
func buildGenericMatchGraph(fn genericFunc, ref defsyntax.Node) *state {
	return &state{kind: stateMatchGraph, match: &state{kind: stateGeneric, generic: fn}, syntax: ref}
}
