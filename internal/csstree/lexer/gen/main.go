// Command gen extracts the parts of mdn-data 2.0.28 and css-tree 2.2.0's
// data/patch.json that the lexer reads (syntax strings only) into one compact
// JSON file that the lexer package embeds.
//
// Usage (see the go:generate directive in data.go):
//
//	go run ./gen -modules ../../../node_modules -out mdndata.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// syntaxEntry is the only field of an mdn-data property or syntax entry, or
// of a patch entry, that css-tree uses. A patch entry without a syntax
// deletes the original definition, so Syntax keeps its absence.
type syntaxEntry struct {
	Syntax *string `json:"syntax,omitempty"`
}

type mdnAtrule struct {
	Syntax      string                 `json:"syntax"`
	Descriptors map[string]syntaxEntry `json:"descriptors,omitempty"`
}

type patchAtrule struct {
	Prelude     *string                `json:"prelude,omitempty"`
	Descriptors map[string]syntaxEntry `json:"descriptors,omitempty"`
}

type output struct {
	Properties map[string]string    `json:"properties"`
	Syntaxes   map[string]string    `json:"syntaxes"`
	Atrules    map[string]mdnAtrule `json:"atrules"`
	Patch      struct {
		Types      map[string]syntaxEntry `json:"types"`
		Properties map[string]syntaxEntry `json:"properties"`
		Atrules    map[string]patchAtrule `json:"atrules"`
	} `json:"patch"`
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// syntaxesOf keeps the syntax of every entry of an mdn-data dictionary.
func syntaxesOf(path string) (map[string]string, error) {
	var dict map[string]syntaxEntry
	if err := readJSON(path, &dict); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(dict))
	for name, entry := range dict {
		if entry.Syntax == nil {
			return nil, fmt.Errorf("%s: %q has no syntax", path, name)
		}
		result[name] = *entry.Syntax
	}
	return result, nil
}

func run(modules, out string) error {
	var data output
	var err error

	mdn := filepath.Join(modules, "mdn-data", "css")
	if data.Properties, err = syntaxesOf(filepath.Join(mdn, "properties.json")); err != nil {
		return err
	}
	if data.Syntaxes, err = syntaxesOf(filepath.Join(mdn, "syntaxes.json")); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(mdn, "at-rules.json"), &data.Atrules); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(modules, "css-tree", "data", "patch.json"), &data.Patch); err != nil {
		return err
	}

	// keep `<`, `>` and `&` readable: they occur in nearly every syntax
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

func main() {
	modules := flag.String("modules", "node_modules", "node_modules directory holding mdn-data and css-tree")
	out := flag.String("out", "mdndata.json", "output file")
	flag.Parse()

	if err := run(*modules, *out); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}
