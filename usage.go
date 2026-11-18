package csso

import (
	"github.com/css/csso-go/internal/csso/usage"
)

// Usage describes the markup a stylesheet is used with (csso's `usage`
// option). A nil slice means "no information" for that kind of name, while
// an empty, non-nil slice means that no such names are used at all.
type Usage struct {
	// Tags are the element names in use, compared case-insensitively.
	Tags []string
	// IDs are the ids in use.
	IDs []string
	// Classes are the class names in use.
	Classes []string
	// Scopes groups class names into scopes: selectors with classes of
	// different scopes never match the same element, so their rules may be
	// reordered. A class may belong to one scope only; a nil scope is
	// invalid.
	Scopes [][]string
	// Blacklist names tags, ids and classes whose selectors are removed.
	Blacklist *UsageBlacklist
}

// UsageBlacklist lists names whose selectors are removed. As in Usage, a
// nil slice means "no names of that kind".
type UsageBlacklist struct {
	Tags    []string
	IDs     []string
	Classes []string
}

// index validates the usage data and builds csso's usage index from it.
func (u *Usage) index() (*usage.Index, error) {
	scopes, err := usage.NewScopes(u.Scopes)
	if err != nil {
		return nil, convertError(err)
	}

	index := &usage.Index{
		Whitelist: usage.NewList(u.Tags, u.IDs, u.Classes),
		Scopes:    scopes,
	}
	if u.Blacklist != nil {
		index.Blacklist = usage.NewList(u.Blacklist.Tags, u.Blacklist.IDs, u.Blacklist.Classes)
	}
	return index, nil
}
