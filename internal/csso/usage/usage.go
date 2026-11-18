// Package usage builds the usage index csso filters and scopes selectors with
// (csso lib/usage.js).
package usage

import (
	"github.com/css/csso-go/internal/csso/jscompat"
	"github.com/css/csso-go/internal/csstree"
)

// Names is a set of names. A nil Names stands for csso's null map: no
// restriction for that kind of selector.
type Names map[string]bool

// List holds the names of the tags, ids and classes of a whitelist or
// blacklist.
type List struct {
	Tags    Names
	IDs     Names
	Classes Names
}

// Index is csso's processed usage data.
type Index struct {
	// Whitelist and Blacklist are nil when no names of any kind were given.
	Whitelist *List
	Blacklist *List
	// Scopes maps a class name to its one-based scope number; nil when no
	// scopes were given.
	Scopes map[string]int
}

// buildNames is csso's buildMap: nil input means "not an array" and yields
// a nil set; tag names are matched case-insensitively.
func buildNames(names []string, caseInsensitive bool) Names {
	if names == nil {
		return nil
	}
	set := make(Names, len(names))
	for _, name := range names {
		if caseInsensitive {
			name = csstree.LowerCase(name)
		}
		set[name] = true
	}
	return set
}

// NewList builds a whitelist or blacklist (csso's buildList). It returns nil
// when tags, ids and classes are all nil.
func NewList(tags, ids, classes []string) *List {
	l := &List{
		Tags:    buildNames(tags, true),
		IDs:     buildNames(ids, false),
		Classes: buildNames(classes, false),
	}
	if l.Tags == nil && l.IDs == nil && l.Classes == nil {
		return nil
	}
	return l
}

// NewScopes numbers the classes of each scope (the scopes part of csso's
// buildIndex). A nil scope is csso's "Wrong usage format" error, and a class
// listed in two scopes is rejected.
func NewScopes(scopes [][]string) (map[string]int, error) {
	if scopes == nil {
		return nil, nil
	}
	index := make(map[string]int)
	for i, list := range scopes {
		if list == nil {
			return nil, jscompat.NewError("Wrong usage format")
		}
		for _, name := range list {
			if _, ok := index[name]; ok {
				return nil, jscompat.NewError("Class can't be used for several scopes: " + name)
			}
			index[name] = i + 1
		}
	}
	return index, nil
}
