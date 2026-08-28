package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The schema document is the authored input contract; the compiler is the
// enforced one. They drifted four separate times in one sitting -- witness
// order, the relational pattern, the `timeout` outcome, two diagnostics, and
// an entire undocumented `Universe:` directive that spec-lint accepts but the
// proof requires. Each was found by hand, after specs had already been written
// against the stale text.
//
// A checksum freeze cannot catch this: it detects that the document changed,
// not that the compiler did. These tests compare the document against the
// compiler's own declarations, so drift fails the build instead of surfacing
// as a task that compiles and then cannot be proved.

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve this file's path")
	}
	return filepath.Dir(filepath.Dir(thisFile))
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(body)
}

// matches returns the deduplicated, sorted capture group 1 of every match.
func matches(pattern, text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

func missing(want []string, gotText string) []string {
	var out []string
	for _, item := range want {
		if !strings.Contains(gotText, item) {
			out = append(out, item)
		}
	}
	return out
}

const schemaDoc = "skills/spec/references/schema.md"

// Every table column the compiler requires must appear in the schema doc.
func TestSchemaDoc_DocumentsEveryColumn(t *testing.T) {
	compiler := readRepoFile(t, "internal/speccompiler/compiler.go")
	doc := readRepoFile(t, schemaDoc)

	columns := matches(`Header[A-Za-z]+ *= *"([^"]+)"`, compiler)
	if len(columns) < 10 {
		t.Fatalf("expected the compiler to declare the column set, found %v", columns)
	}
	if absent := missing(columns, doc); len(absent) > 0 {
		t.Errorf("%s does not document these columns the compiler requires: %v", schemaDoc, absent)
	}
}

// Every document-level directive the compiler parses must be documented.
// `Universe:` was accepted by the compiler and absent from the doc; specs
// written without it compiled and would have failed at proof time.
func TestSchemaDoc_DocumentsEveryDirective(t *testing.T) {
	grounding := readRepoFile(t, "internal/speccompiler/grounding.go")
	doc := readRepoFile(t, schemaDoc)

	directives := matches(`HasPrefix\(trimmed, "([A-Za-z]+:)"\)`, grounding)
	if len(directives) == 0 {
		t.Fatal("expected the compiler to parse at least one directive")
	}
	if absent := missing(directives, doc); len(absent) > 0 {
		t.Errorf("%s does not document these directives the compiler parses: %v", schemaDoc, absent)
	}
}

// Every literal outcome keyword the compiler accepts must be documented, or an
// author cannot know it exists. `timeout` was missing from the skill's list.
func TestSchemaDoc_DocumentsEveryOutcomeKeyword(t *testing.T) {
	compiler := readRepoFile(t, "internal/speccompiler/compiler.go")
	doc := readRepoFile(t, schemaDoc)

	// The outcome switch compares the cell text against each bare keyword.
	keywords := matches(`case text == "([a-z ]+)"`, compiler)
	if len(keywords) == 0 {
		t.Fatal("expected the compiler to accept at least one outcome keyword")
	}
	if absent := missing(keywords, doc); len(absent) > 0 {
		t.Errorf("%s does not document these outcome forms the compiler accepts: %v", schemaDoc, absent)
	}
}

// Every effect kind must be documented; an undocumented kind is unusable.
func TestSchemaDoc_DocumentsEveryEffectKind(t *testing.T) {
	types := readRepoFile(t, "internal/semanticir/types.go")
	doc := readRepoFile(t, schemaDoc)

	kinds := matches(`Effect(?:Read|Write|Call|Output) +EffectKind += +"([a-z]+)"`, types)
	if len(kinds) == 0 {
		t.Fatal("expected the IR to declare effect kinds")
	}
	if absent := missing(kinds, doc); len(absent) > 0 {
		t.Errorf("%s does not document these effect kinds: %v", schemaDoc, absent)
	}
}

// Every diagnostic an author can receive must be documented with its cause,
// otherwise the message is the only explanation they get.
func TestSchemaDoc_DocumentsEveryDiagnostic(t *testing.T) {
	types := readRepoFile(t, "internal/semanticir/types.go")
	doc := readRepoFile(t, schemaDoc)

	// Diagnostic<Name> constants carry a kebab-case wire value.
	codes := matches(`Diagnostic[A-Za-z]+ +DiagnosticCode += +"([a-z-]+)"`, types)
	if len(codes) == 0 {
		t.Fatal("expected the IR to declare diagnostic codes")
	}
	if absent := missing(codes, doc); len(absent) > 0 {
		t.Errorf("%s does not document these diagnostics the compiler can emit: %v", schemaDoc, absent)
	}
}
