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

	// A stress test found a real, dangerous bug here: requiring the
	// paragraph to START WITH "Parameters:" meant a lead-in sentence
	// sharing the same paragraph (no blank line before "Parameters:")
	// made the ENTIRE domain declaration silently vanish -- not corrupt,
	// gone -- and spec-lint then reported a clean PASS on a table with
	// an undeclared cell value, since there was no domain left to check
	// it against. Matching on Contains instead of HasPrefix keeps the
	// declaration regardless of what prose precedes it in the same
	// paragraph; ParseParams already finds and strips everything before
	// "Parameters:" itself, so passing the whole paragraph through is
	// safe once it's no longer being discarded first.
	flushParaAsParams := func() string {
		para := strings.TrimSpace(strings.Join(paraBuf, " "))
		paraBuf = nil
		if strings.Contains(para, "Parameters:") {
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
				if !strings.Contains(strings.Join(paraBuf, " "), "Parameters:") {
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

// truncateAtDeclarationEnd returns s up to (excluding) the first "."
// that occurs outside both a backtick-quoted name and a parenthesized
// value group -- the declaration sentence's own real end. If no such
// period exists, s is returned unchanged (a single-sentence Parameters
// line with no trailing prose to exclude).
func truncateAtDeclarationEnd(s string) string {
	depth := 0
	inBacktick := false
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if s[i] == '\\' {
				escaped = true
				continue
			}
			if s[i] == '"' {
				inQuote = false
			}
			continue
		}
		switch s[i] {
		case '`':
			inBacktick = !inBacktick
		case '"':
			if !inBacktick {
				inQuote = true
			}
		case '(':
			if !inBacktick {
				depth++
			}
		case ')':
			if !inBacktick && depth > 0 {
				depth--
			}
		case '.':
			if !inBacktick && depth == 0 {
				return s[:i]
			}
		}
	}
	return s
}

// Domain is one parameter's declared name and its disjoint set of
// legal values, as declared in a Parameters: line.
type Domain struct {
	Name       string
	Values     []string
	JSONQuoted map[string]bool
}

// ParseParams parses "Parameters: `name` (v1 / v2), `name2` (v3 / v4)."
// into one Domain per parameter, in declared order. If a parameter's
// parenthesized group contains no unquoted " / " separator, it wasn't decomposed into
// disjoint categorical values and its name is returned as unsupported.
//
// A real stress test found that when the Parameters: sentence shares a
// Markdown paragraph with trailing explanatory prose (no blank line
// between them -- e.g. "Parameters: `x` (a / b). This means `a` (fast)
// or `b` (slow/legacy)."), the whole paragraph was being scanned, so
// backticks and "/" inside that trailing prose were misread as more
// parameter declarations. The declaration sentence itself always ends
// at its own top-level period (one not nested inside a paren or a
// backtick pair, since the format is "Parameters: ... (...), ....") --
// truncating there before parsing keeps trailing prose out of scope
// entirely, rather than special-casing backticks or "/" individually.
func ParseParams(raw string) (doms []Domain, unsupported string, err error) {
	s := raw
	if idx := strings.Index(s, "Parameters:"); idx >= 0 {
		s = s[idx+len("Parameters:"):]
	}
	s = truncateAtDeclarationEnd(s)

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
		inQuote := false
		escaped := false
		for p := parenStart; p < len(rest); p++ {
			if inQuote {
				if escaped {
					escaped = false
					continue
				}
				if rest[p] == '\\' {
					escaped = true
					continue
				}
				if rest[p] == '"' {
					inQuote = false
				}
				continue
			}
			switch rest[p] {
			case '"':
				inQuote = true
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
		content = truncateOutsideJSONString(content, "—")
		decoded, compound, parseErr := ParseFiniteValues(content)
		if parseErr != nil {
			return nil, "", fmt.Errorf("parameter %q: %w", name, parseErr)
		}
		if len(decoded) == 1 && !compound && !decoded[0].JSONQuoted {
			return nil, name, nil
		}
		domain := Domain{Name: name, JSONQuoted: map[string]bool{}}
		for _, value := range decoded {
			domain.Values = append(domain.Values, value.Value)
			domain.JSONQuoted[value.Value] = value.JSONQuoted
		}
		doms = append(doms, domain)

		i = end + 1 + parenEnd + 1
	}
	return doms, "", nil
}
