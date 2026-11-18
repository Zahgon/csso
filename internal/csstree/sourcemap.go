package csstree

import (
	"slices"
	"strconv"
	"strings"
)

// This file ports css-tree's lib/generator/sourceMap.js and the parts of
// source-map-js 1.0.1 (SourceMapGenerator, MappingList, base64-vlq) that it
// uses.

// sourceMapTracker follows the generated position while generating and
// records a mapping whenever an Atrule, Selector or Declaration with a
// location starts at a new original position.
type sourceMapTracker struct {
	sourceMap *SourceMap

	line   int // current generated line (1-based)
	column int // current generated column (0-based, UTF-16 units)

	originalLine   int
	originalColumn int

	// activated is the generated position reached at the end of the last
	// tracked node; it is closed with a source-less mapping when the next
	// tracked node starts elsewhere.
	active          bool
	activatedLine   int
	activatedColumn int
}

func newSourceMapTracker() *sourceMapTracker {
	return &sourceMapTracker{
		sourceMap:     NewSourceMap(),
		line:          1,
		activatedLine: 1,
	}
}

func isTrackedNode(n Node) bool {
	switch n.(type) {
	case *Atrule, *Selector, *Declaration:
		return true
	}
	return false
}

func (t *sourceMapTracker) enterNode(n Node) {
	loc := n.Loc()
	if loc == nil || !isTrackedNode(n) {
		return
	}
	nodeLine := loc.Start.Line
	nodeColumn := loc.Start.Column - 1
	if t.originalLine == nodeLine && t.originalColumn == nodeColumn {
		return
	}
	t.originalLine = nodeLine
	t.originalColumn = nodeColumn

	if t.active {
		t.active = false
		if t.line != t.activatedLine || t.column != t.activatedColumn {
			t.sourceMap.addMapping(mapping{generatedLine: t.activatedLine, generatedColumn: t.activatedColumn})
		}
	}

	t.active = true
	t.sourceMap.addMapping(mapping{
		generatedLine:   t.line,
		generatedColumn: t.column,
		hasSource:       true,
		source:          loc.Source,
		originalLine:    nodeLine,
		originalColumn:  nodeColumn,
	})
}

func (t *sourceMapTracker) leaveNode(n Node) {
	if t.active && isTrackedNode(n) {
		t.activatedLine = t.line
		t.activatedColumn = t.column
	}
}

// advance moves the generated position past value.
func (t *sourceMapTracker) advance(value string) {
	for _, r := range value {
		switch {
		case r == '\n':
			t.line++
			t.column = 0
		case r >= 0x10000:
			t.column += 2
		default:
			t.column++
		}
	}
}

func (t *sourceMapTracker) finish() *SourceMap {
	if t.active {
		t.sourceMap.addMapping(mapping{generatedLine: t.activatedLine, generatedColumn: t.activatedColumn})
	}
	return t.sourceMap
}

// mapping is a source-map-js mapping. A mapping without source has its
// original position read as 0 (JavaScript's false) when sorting.
type mapping struct {
	generatedLine   int
	generatedColumn int
	hasSource       bool
	source          string
	originalLine    int
	originalColumn  int
}

// SourceMap is a version 3 source map under construction, reproducing
// source-map-js's SourceMapGenerator output.
type SourceMap struct {
	// File is the "file" field; it is omitted from the output when empty.
	File string

	sources        []string
	mappings       []mapping
	sorted         bool
	last           mapping
	sourcesContent map[string]string
}

// NewSourceMap returns an empty source map.
func NewSourceMap() *SourceMap {
	return &SourceMap{sorted: true, last: mapping{generatedLine: -1}}
}

// Sources returns the source file names, in order of first use.
func (m *SourceMap) Sources() []string {
	return slices.Clone(m.sources)
}

// SetSourceContent records the content of a source file, emitted in
// "sourcesContent".
func (m *SourceMap) SetSourceContent(source, content string) {
	if m.sourcesContent == nil {
		m.sourcesContent = make(map[string]string)
	}
	m.sourcesContent[source] = content
}

func (m *SourceMap) addMapping(mp mapping) {
	if mp.hasSource && !slices.Contains(m.sources, mp.source) {
		m.sources = append(m.sources, mp.source)
	}
	// MappingList keeps track of whether insertion order is sorted
	if generatedPositionAfter(m.last, mp) {
		m.last = mp
	} else {
		m.sorted = false
	}
	m.mappings = append(m.mappings, mp)
}

func generatedPositionAfter(a, b mapping) bool {
	return b.generatedLine > a.generatedLine ||
		(b.generatedLine == a.generatedLine && b.generatedColumn >= a.generatedColumn) ||
		compareMappings(a, b) <= 0
}

// compareMappings is source-map-js's compareByGeneratedPositionsInflated.
func compareMappings(a, b mapping) int {
	if d := a.generatedLine - b.generatedLine; d != 0 {
		return d
	}
	if d := a.generatedColumn - b.generatedColumn; d != 0 {
		return d
	}
	if d := compareSources(a, b); d != 0 {
		return d
	}
	if d := a.originalLine - b.originalLine; d != 0 {
		return d
	}
	return a.originalColumn - b.originalColumn
}

// compareSources orders mappings without source after those with one.
func compareSources(a, b mapping) int {
	switch {
	case a.hasSource == b.hasSource && (!a.hasSource || a.source == b.source):
		return 0
	case !a.hasSource:
		return 1
	case !b.hasSource:
		return -1
	}
	return strings.Compare(a.source, b.source)
}

func (m *SourceMap) sortedMappings() []mapping {
	if !m.sorted {
		slices.SortStableFunc(m.mappings, compareMappings)
		m.sorted = true
	}
	return m.mappings
}

// Mappings returns the encoded "mappings" field.
func (m *SourceMap) Mappings() string {
	var b strings.Builder
	prevGeneratedColumn := 0
	prevGeneratedLine := 1
	prevOriginalColumn := 0
	prevOriginalLine := 0
	prevSource := 0

	mappings := m.sortedMappings()
	for i, mp := range mappings {
		if mp.generatedLine != prevGeneratedLine {
			prevGeneratedColumn = 0
			for mp.generatedLine != prevGeneratedLine {
				b.WriteByte(';')
				prevGeneratedLine++
			}
		} else if i > 0 {
			if compareMappings(mp, mappings[i-1]) == 0 {
				continue
			}
			b.WriteByte(',')
		}

		writeVLQ(&b, mp.generatedColumn-prevGeneratedColumn)
		prevGeneratedColumn = mp.generatedColumn

		if mp.hasSource {
			sourceIdx := slices.Index(m.sources, mp.source)
			writeVLQ(&b, sourceIdx-prevSource)
			prevSource = sourceIdx

			writeVLQ(&b, mp.originalLine-1-prevOriginalLine)
			prevOriginalLine = mp.originalLine - 1

			writeVLQ(&b, mp.originalColumn-prevOriginalColumn)
			prevOriginalColumn = mp.originalColumn
		}
	}
	return b.String()
}

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// writeVLQ appends the base64 VLQ encoding of value.
func writeVLQ(b *strings.Builder, value int) {
	vlq := value << 1
	if value < 0 {
		vlq = (-value << 1) + 1
	}
	for {
		digit := vlq & 31
		vlq >>= 5
		if vlq > 0 {
			digit |= 32
		}
		b.WriteByte(base64Chars[digit])
		if vlq <= 0 {
			return
		}
	}
}

// String returns the source map as JSON, byte-for-byte as source-map-js's
// toString() (JSON.stringify of toJSON()).
func (m *SourceMap) String() string {
	var b strings.Builder
	b.WriteString(`{"version":3,"sources":[`)
	for i, source := range m.sources {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSONString(&b, source)
	}
	b.WriteString(`],"names":[],"mappings":`)
	writeJSONString(&b, m.Mappings())
	if m.File != "" {
		b.WriteString(`,"file":`)
		writeJSONString(&b, m.File)
	}
	if m.sourcesContent != nil {
		b.WriteString(`,"sourcesContent":[`)
		for i, source := range m.sources {
			if i > 0 {
				b.WriteByte(',')
			}
			if content, ok := m.sourcesContent[source]; ok {
				writeJSONString(&b, content)
			} else {
				b.WriteString("null")
			}
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

// writeJSONString writes s quoted the way JSON.stringify does: only the
// quote, the reverse solidus and control characters are escaped.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteString(strconv.FormatInt(int64(r)>>4, 16))
				b.WriteString(strconv.FormatInt(int64(r)&15, 16))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
