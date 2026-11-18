// Package restructure implements csso's structural optimizations: merging
// at-rules and rules, splitting and rejoining selectors, building
// shorthands and moving declarations between rules (csso lib/restructure).
package restructure

import (
	"github.com/css/csso-go/internal/csso/usage"
	"github.com/css/csso-go/internal/csstree"
)

// Options configures Run.
type Options struct {
	// Usage provides the class scopes; nil when no usage data was given.
	Usage *usage.Index
	// ForceMediaMerge groups all @media rules with the same query, even
	// when that changes the order of rules.
	ForceMediaMerge bool
	// Logger, when set, is called with the name of each step and the
	// stylesheet after it.
	Logger func(title string, ast csstree.Node)
}

// Run restructures the stylesheet ast in place.
func Run(ast *csstree.StyleSheet, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = func(string, csstree.Node) {}
	}

	// prepare ast for restructuring
	indexer, err := prepare(ast, opts.Usage)
	if err != nil {
		return err
	}
	logger("prepare", ast)

	mergeAtrule(ast, opts.ForceMediaMerge)
	logger("mergeAtrule", ast)

	initialMergeRuleset(ast)
	logger("initialMergeRuleset", ast)

	disjoinRuleset(ast)
	logger("disjoinRuleset", ast)

	restructShorthand(ast, indexer)
	logger("restructShorthand", ast)

	if err := restructBlock(ast); err != nil {
		return err
	}
	logger("restructBlock", ast)

	mergeRuleset(ast)
	logger("mergeRuleset", ast)

	restructRuleset(ast)
	logger("restructRuleset", ast)

	return nil
}
