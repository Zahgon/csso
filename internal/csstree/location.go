package csstree

// offsetToLocation is css-tree's OffsetToLocation: it maps source offsets to
// one-based line and column numbers, computed lazily on first use.
type offsetToLocation struct {
	source      []uint16
	startOffset int
	startLine   int
	startColumn int
	lines       []int
	columns     []int
}

func (m *offsetToLocation) setSource(source []uint16, startOffset, startLine, startColumn int) {
	m.source = source
	m.startOffset = startOffset
	m.startLine = startLine
	m.startColumn = startColumn
	m.lines = nil
	m.columns = nil
}

// compute fills the line/column tables. CR LF counts as a single line
// break; a leading byte order mark is not given a position (css-tree leaves
// that slot of its zero-initialized buffers untouched).
func (m *offsetToLocation) compute() {
	n := len(m.source)
	lines := make([]int, n+1)
	columns := make([]int, n+1)
	start := 0
	if n > 0 {
		start = isBOM(int(m.source[0]))
	}
	line := m.startLine
	column := m.startColumn
	for i := start; i < n; i++ {
		code := m.source[i]
		lines[i] = line
		columns[i] = column
		column++
		if code == '\n' || code == '\r' || code == '\f' {
			if code == '\r' && i+1 < n && m.source[i+1] == '\n' {
				i++
				lines[i] = line
				columns[i] = column
			}
			line++
			column = 1
		}
	}
	lines[n] = line
	columns[n] = column
	m.lines = lines
	m.columns = columns
}

// position returns the location of offset. Offsets past the end (which
// css-tree can produce for a bad URL at the end of input) are clamped.
func (m *offsetToLocation) position(offset int) Position {
	if m.lines == nil {
		m.compute()
	}
	index := min(offset, len(m.source))
	return Position{
		Offset: m.startOffset + offset,
		Line:   m.lines[index],
		Column: m.columns[index],
	}
}

func (m *offsetToLocation) locationRange(start, end int, filename string) *Location {
	return &Location{
		Source: filename,
		Start:  m.position(start),
		End:    m.position(end),
	}
}
