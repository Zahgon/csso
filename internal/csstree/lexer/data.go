package lexer

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:generate go run ./gen -modules ../../../node_modules -out mdndata.json

// mdnDataJSON holds the syntax strings of mdn-data 2.0.28 (css/properties,
// css/syntaxes and css/at-rules) together with css-tree 2.2.0's
// data/patch.json, as extracted by ./gen.
//
//go:embed mdndata.json
var mdnDataJSON []byte

// Definitions is the syntax dictionary a Lexer is built from: css-tree's
// data.js output (mdn-data with css-tree's patch applied).
type Definitions struct {
	Types      map[string]string
	Properties map[string]string
	Atrules    map[string]AtruleDefinition
}

// AtruleDefinition is the syntax of an at-rule. Prelude is empty when the
// at-rule takes no prelude. Descriptors is nil when the at-rule has no
// descriptor dictionary at all, as opposed to an empty one.
type AtruleDefinition struct {
	Prelude     string
	Descriptors map[string]string
}

// syntaxEntry is an mdn-data or patch entry reduced to its syntax; a patch
// entry without one deletes the original definition.
type syntaxEntry struct {
	Syntax *string `json:"syntax"`
}

// mdnData is the layout of mdndata.json (see ./gen).

type mdnData struct {
	Properties map[string]string `json:"properties"`
	Syntaxes   map[string]string `json:"syntaxes"`
	Atrules    map[string]struct {
		Syntax      string                 `json:"syntax"`
		Descriptors map[string]syntaxEntry `json:"descriptors"`
	} `json:"atrules"`
	Patch struct {
		Types      map[string]syntaxEntry `json:"types"`
		Properties map[string]syntaxEntry `json:"properties"`
		Atrules    map[string]struct {
			// patch.json never sets a null prelude, so a nil Prelude means
			// the key is absent
			Prelude     *string                `json:"prelude"`
			Descriptors map[string]syntaxEntry `json:"descriptors"`
		} `json:"atrules"`
	} `json:"patch"`
}

// extendSyntax matches a patch syntax that extends the original one
// (`| <extra>`) instead of replacing it.
var extendSyntax = regexp.MustCompile(`^\s*\|\s*`)

// atruleBlock and atrulePrelude extract the prelude from an mdn-data at-rule
// syntax such as "@media <media-query-list> {\n  <group-rule-body>\n}".
var (
	atruleBlock   = regexp.MustCompile(`(?s)\{.+\}`)
	atrulePrelude = regexp.MustCompile(`^@\S+\s+([^;{]*)`)
)

// MDNDefinitions returns the definitions css-tree 2.2.0 builds from mdn-data
// 2.0.28 and its own data patch.
func MDNDefinitions() (*Definitions, error) {
	var data mdnData
	if err := json.Unmarshal(mdnDataJSON, &data); err != nil {
		return nil, fmt.Errorf("lexer: decoding embedded mdn-data: %w", err)
	}

	atrules := make(map[string]AtruleDefinition, len(data.Atrules))
	for name, atrule := range data.Atrules {
		syntax := atruleBlock.ReplaceAllLiteralString(strings.TrimSpace(atrule.Syntax), "")
		m := atrulePrelude.FindStringSubmatch(syntax)
		if m == nil {
			return nil, fmt.Errorf("lexer: at-rule %s: no prelude in syntax %q", name, atrule.Syntax)
		}
		def := AtruleDefinition{Prelude: strings.TrimSpace(m[1])}
		if atrule.Descriptors != nil {
			def.Descriptors = make(map[string]string, len(atrule.Descriptors))
			for desc, entry := range atrule.Descriptors {
				if entry.Syntax != nil {
					def.Descriptors[desc] = *entry.Syntax
				}
			}
		}
		atrules[strings.TrimPrefix(name, "@")] = def
	}

	defs := &Definitions{
		Types:      patchDictionary(data.Syntaxes, data.Patch.Types),
		Properties: patchDictionary(data.Properties, data.Patch.Properties),
		Atrules:    make(map[string]AtruleDefinition, len(atrules)),
	}

	for name, atrule := range atrules {
		patch, patched := data.Patch.Atrules[name]
		def := AtruleDefinition{Prelude: atrule.Prelude}
		if patched && patch.Prelude != nil {
			def.Prelude = *patch.Prelude
		}
		// css-tree always gives a known at-rule a descriptor dictionary,
		// possibly an empty one
		def.Descriptors = patchDictionary(atrule.Descriptors, patch.Descriptors)
		defs.Atrules[name] = def
	}
	for name, patch := range data.Patch.Atrules {
		if _, known := atrules[name]; known {
			continue
		}
		def := AtruleDefinition{}
		if patch.Prelude != nil {
			def.Prelude = *patch.Prelude
		}
		if patch.Descriptors != nil {
			def.Descriptors = patchDictionary(nil, patch.Descriptors)
		}
		defs.Atrules[name] = def
	}

	return defs, nil
}

// patchDictionary applies css-tree's patch to an mdn-data dictionary: a patch
// syntax starting with `|` extends the original syntax, any other syntax
// replaces it (or adds a new entry), and a patch entry without a syntax
// removes the original one.
func patchDictionary(dict map[string]string, patch map[string]syntaxEntry) map[string]string {
	result := make(map[string]string, len(dict)+len(patch))
	for name, syntax := range dict {
		result[name] = syntax
	}

	for name, entry := range patch {
		_, known := dict[name]
		switch {
		case known && (entry.Syntax == nil || *entry.Syntax == ""):
			delete(result, name)
		case known && extendSyntax.MatchString(*entry.Syntax):
			result[name] += " " + strings.TrimSpace(*entry.Syntax)
		case known:
			result[name] = *entry.Syntax
		case entry.Syntax != nil && *entry.Syntax != "":
			result[name] = extendSyntax.ReplaceAllLiteralString(*entry.Syntax, "")
		}
	}

	return result
}
