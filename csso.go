// Package csso is a Go port of csso 5.0.5, the CSS minifier: it parses CSS
// with csso's fork of css-tree, cleans it, replaces values by shorter
// equivalents and restructures rules, producing the same output as the
// JavaScript original.
//
//	result, err := csso.Minify(".a { color: #ff0000; } .b { color: red }")
//	// result.CSS == ".a,.b{color:red}"
package csso

import (
	"github.com/css/csso-go/internal/csstree"
)

// Comments selects which comments survive minification (csso's `comments`
// option). Only "exclamation" comments (`/*! ... */`) can be kept, and only
// at the top level of a stylesheet.
type Comments int

const (
	// CommentsExclamation keeps every top-level exclamation comment. It is
	// the default, as in csso.
	CommentsExclamation Comments = iota
	// CommentsFirstExclamation keeps only the first exclamation comment,
	// and only when it precedes all rules.
	CommentsFirstExclamation
	// CommentsNone removes all comments.
	CommentsNone
)

// Options configures Minify, MinifyBlock and Compress. The zero value is
// csso's default configuration.
type Options struct {
	// Restructure enables structural optimizations (merging and moving
	// rules and declarations); nil means true. This is csso's
	// `restructure` option (formerly `restructuring`).
	Restructure *bool
	// ForceMediaMerge merges all @media rules with the same query into one,
	// even though that may change the meaning of the stylesheet.
	ForceMediaMerge bool
	// Comments selects the comments to keep.
	Comments Comments
	// Usage, when set, describes the tags, ids and classes in use: other
	// selectors are removed and classes of different scopes are never
	// merged.
	Usage *Usage
	// SourceMap makes Minify return a source map in Result.Map.
	SourceMap bool
	// Filename is the source name used in node locations and the source
	// map; "" means "<unknown>".
	Filename string
	// Clone makes Compress work on a copy of the tree instead of changing
	// it in place.
	Clone bool
	// Logger, when set, is called during compression with the name of each
	// stage and the stylesheet chunk after it; ast is nil for the message
	// announcing a new chunk. (csso's `debug` option, which logs timings to
	// standard error, is not supported.)
	Logger func(title string, ast *AST)
	// BeforeCompress and AfterCompress are called by Minify and
	// MinifyBlock with the tree right after parsing and right after
	// compression; an error aborts the minification.
	BeforeCompress []func(ast *AST, opts Options) error
	AfterCompress  []func(ast *AST, opts Options) error
}

// restructure reports whether structural optimizations are enabled.
func (o *Options) restructure() bool {
	return o.Restructure == nil || *o.Restructure
}

// filename returns the source name, defaulting to "<unknown>".
func (o *Options) filename() string {
	if o.Filename == "" {
		return "<unknown>"
	}
	return o.Filename
}

// optionsOf returns the options of a variadic options argument: the first
// one given, or the defaults.
func optionsOf(opts []Options) Options {
	if len(opts) > 0 {
		return opts[0]
	}
	return Options{}
}

// Result is the result of Minify and MinifyBlock.
type Result struct {
	// CSS is the minified CSS.
	CSS string
	// Map is the source map, or nil unless Options.SourceMap is set.
	Map *SourceMap
}

// SourceMap is a version 3 source map of minified CSS.
type SourceMap struct {
	m *csstree.SourceMap
}

// String returns the source map as JSON, exactly as csso's
// `map.toString()` (source-map-js) renders it.
func (m *SourceMap) String() string {
	return m.m.String()
}

// Minify minifies a stylesheet (csso.minify). opts is optional; only its
// first element is used.
func Minify(source string, opts ...Options) (Result, error) {
	return minify(csstree.ContextStylesheet, source, optionsOf(opts))
}

// MinifyBlock minifies a list of declarations, such as the content of a
// style attribute (csso.minifyBlock). opts is optional; only its first
// element is used.
func MinifyBlock(source string, opts ...Options) (Result, error) {
	return minify(csstree.ContextDeclarationList, source, optionsOf(opts))
}

func minify(context, source string, opts Options) (Result, error) {
	filename := opts.filename()

	node, err := csstree.Parse(source, &csstree.ParseOptions{
		Context:   context,
		Filename:  filename,
		Positions: opts.SourceMap,
	})
	if err != nil {
		return Result{}, convertError(err)
	}

	if err := runHandlers(opts.BeforeCompress, &AST{node: node}, opts); err != nil {
		return Result{}, err
	}

	node, err = compress(node, opts)
	if err != nil {
		return Result{}, err
	}

	if err := runHandlers(opts.AfterCompress, &AST{node: node}, opts); err != nil {
		return Result{}, err
	}

	if !opts.SourceMap {
		return Result{CSS: csstree.Generate(node)}, nil
	}

	css, sourceMap := csstree.GenerateWithSourceMap(node)
	// other tools in a source map transform chain may rely on the file
	sourceMap.File = filename
	sourceMap.SetSourceContent(filename, source)
	return Result{CSS: css, Map: &SourceMap{m: sourceMap}}, nil
}

func runHandlers(handlers []func(*AST, Options) error, ast *AST, opts Options) error {
	for _, handler := range handlers {
		if err := handler(ast, opts); err != nil {
			return err
		}
	}
	return nil
}
