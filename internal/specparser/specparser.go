// Package specparser parses spec.md's Markdown condition tables.
package specparser

import (
	"fmt"
	"strings"
)

type Table struct {
	Section string
	Params  string
	Columns []string
	Rows    [][]string
	Line    int
}

func Parse(content string) ([]Table, error) {
	lines := strings.Split(content, "\n")
	var tables []Table
	currentSection := ""
	var paraBuf []string

	flushParaAsParams := func() string {
		para := strings.TrimSpace(strings.Join(paraBuf, " "))
		paraBuf = nil
		if strings.HasPrefix(para, "Parameters:") {
			return para
		}
		return ""
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if heading, ok := parseHeading(line); ok {
			currentSection = heading
			paraBuf = nil
			continue
		}

		if !looksLikeTableRow(line) {
			if trimmed == "" || trimmed == "---" {
				if !strings.HasPrefix(strings.Join(paraBuf, " "), "Parameters:") {
					paraBuf = nil
				}
			} else {
				paraBuf = append(paraBuf, trimmed)
			}
			continue
		}
		if i+1 >= len(lines) || !isSeparatorRow(lines[i+1]) {
			continue
		}

		params := flushParaAsParams()

		header := splitRow(line)
		var rows [][]string
		j := i + 2
		for ; j < len(lines); j++ {
			if !looksLikeTableRow(lines[j]) {
				break
			}
			row := splitRow(lines[j])
			if len(row) != len(header) {
				return nil, fmt.Errorf(
					"line %d: row has %d cells, header has %d",
					j+1, len(row), len(header),
				)
			}
			rows = append(rows, row)
		}

		tables = append(tables, Table{
			Section: currentSection,
			Params:  params,
			Columns: header,
			Rows:    rows,
			Line:    i + 1,
		})
		i = j - 1
	}

	return tables, nil
}

func parseHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	heading := strings.TrimLeft(trimmed, "#")
	return strings.TrimSpace(heading), true
}

func looksLikeTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && len(trimmed) > 1
}

func isSeparatorRow(line string) bool {
	if !looksLikeTableRow(line) {
		return false
	}
	for _, cell := range splitRow(line) {
		cell = strings.TrimSpace(cell)
		cell = strings.Trim(cell, ":")
		if cell == "" || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func splitRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")

	var cells []string
	for part := range strings.SplitSeq(trimmed, "|") {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// Domain is one parameter's declared name and its disjoint set of
// legal values, as declared in a Parameters: line.
type Domain struct {
	Name   string
	Values []string
}

// ParseParams parses "Parameters: `name` (v1 / v2), `name2` (v3 / v4)."
// into one Domain per parameter, in declared order. If a parameter's
// parenthesized group contains no "/", it wasn't decomposed into
// disjoint categorical values and its name is returned as unsupported.
func ParseParams(raw string) (doms []Domain, unsupported string, err error) {
	s := raw
	if idx := strings.Index(s, "Parameters:"); idx >= 0 {
		s = s[idx+len("Parameters:"):]
	}

	i := 0
	for i < len(s) {
		// Find next backtick-quoted name.
		start := strings.IndexByte(s[i:], '`')
		if start < 0 {
			break
		}
		start += i
		end := strings.IndexByte(s[start+1:], '`')
		if end < 0 {
			return nil, "", fmt.Errorf("unterminated backtick in Parameters line")
		}
		end += start + 1
		name := s[start+1 : end]

		// Find the opening paren after the name.
		rest := s[end+1:]
		parenStart := strings.IndexByte(rest, '(')
		if parenStart < 0 {
			i = end + 1
			continue
		}
		depth := 0
		parenEnd := -1
		for p := parenStart; p < len(rest); p++ {
			switch rest[p] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					parenEnd = p
				}
			}
			if parenEnd >= 0 {
				break
			}
		}
		if parenEnd < 0 {
			return nil, "", fmt.Errorf("unterminated parenthesis for parameter %q", name)
		}
		content := rest[parenStart+1 : parenEnd]
		if idx := strings.Index(content, "—"); idx >= 0 {
			content = content[:idx]
		}

		if !strings.Contains(content, "/") {
			return nil, name, nil
		}

		var values []string
		for _, v := range strings.Split(content, "/") {
			v = strings.TrimSpace(v)
			v = strings.ReplaceAll(v, "`", "")
			if v != "" {
				values = append(values, v)
			}
		}
		doms = append(doms, Domain{Name: name, Values: values})

		i = end + 1 + parenEnd + 1
	}
	return doms, "", nil
}
