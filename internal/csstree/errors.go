package csstree

import (
	"strconv"
	"strings"
)

// SyntaxError is a CSS parse error (css-tree's parser SyntaxError).
type SyntaxError struct {
	Message string
	// Source is the complete parsed source.
	Source string
	// Offset, Line and Column locate the error (one-based line/column,
	// offsets in UTF-16 code units).
	Offset int
	Line   int
	Column int
}

// Error returns the error message.
func (e *SyntaxError) Error() string {
	return e.Message
}

// FormattedMessage returns the message followed by a source excerpt of two
// lines around the error, as css-tree's formattedMessage.
func (e *SyntaxError) FormattedMessage() string {
	return "Parse error: " + e.Message + "\n" + e.SourceFragment(2)
}

const (
	fragmentMaxLineLength    = 100
	fragmentOffsetCorrection = 60
	fragmentTabReplacement   = "    "
)

// SourceFragment renders the source lines around the error with a caret
// under the error column, showing extraLines lines before and after.
func (e *SyntaxError) SourceFragment(extraLines int) string {
	lines := splitLines(toUnits(e.Source))
	line, column := e.Line, e.Column
	startLine := max(1, line-extraLines) - 1
	endLine := min(line+extraLines, len(lines)+1)
	maxNumLength := max(4, len(strconv.Itoa(endLine))) + 1
	cutLeft := 0

	// widen the column by the tabs before it, which are expanded below
	if line-1 >= 0 && line-1 < len(lines) {
		prefix := lines[line-1][:min(max(column-1, 0), len(lines[line-1]))]
		for _, u := range prefix {
			if u == '\t' {
				column += len(fragmentTabReplacement) - 1
			}
		}
	}

	if column > fragmentMaxLineLength {
		cutLeft = column - fragmentOffsetCorrection + 3
		column = fragmentOffsetCorrection - 2
	}

	rendered := make([]string, len(lines))
	for i := range lines {
		rendered[i] = fromUnits(lines[i])
	}
	for i := startLine; i <= endLine; i++ {
		if i < 0 || i >= len(lines) {
			continue
		}
		expanded := toUnits(strings.ReplaceAll(rendered[i], "\t", fragmentTabReplacement))
		var b strings.Builder
		if cutLeft > 0 && len(expanded) > cutLeft {
			b.WriteString("…")
		}
		from := min(cutLeft, len(expanded))
		to := min(from+fragmentMaxLineLength-2, len(expanded))
		b.WriteString(fromUnits(expanded[from:to]))
		if len(expanded) > cutLeft+fragmentMaxLineLength-1 {
			b.WriteString("…")
		}
		rendered[i] = b.String()
	}

	processLines := func(start, end int) string {
		start = max(start, 0)
		end = min(end, len(rendered))
		var out []string
		for i := start; i < end; i++ {
			num := strconv.Itoa(i + 1)
			out = append(out, strings.Repeat(" ", max(maxNumLength-len(num), 0))+num+" |"+rendered[i])
		}
		return strings.Join(out, "\n")
	}

	parts := []string{
		processLines(startLine, line),
		strings.Repeat("-", max(column+maxNumLength+1, 0)) + "^",
		processLines(line, endLine),
	}
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "\n")
}

// splitLines splits src on CR LF, CR, LF and FF.
func splitLines(src []uint16) [][]uint16 {
	var lines [][]uint16
	start := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\r':
			lines = append(lines, src[start:i])
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n', '\f':
			lines = append(lines, src[start:i])
			start = i + 1
		}
	}
	return append(lines, src[start:])
}
