// This check asks the Isla parser to accept generated program text.
// It must use the parser as the authority, never a copy of its field rules.
//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedProgramParses fails when the Isla parser rejects generated
// program text. A rejected program yields an engine error, never a verdict.
func TestGeneratedProgramParses(t *testing.T) {
	if report := parserRejection(t, generatedProgramText(t)); report != "" {
		t.Fatalf("Isla rejected generated program text: %s", report)
	}
}

// TestParserRejectsAnUnknownNestedField proves the parser check can fail, so
// a passing run means the parser accepted the program rather than ignored it.
func TestParserRejectsAnUnknownNestedField(t *testing.T) {
	text := generatedProgramText(t)
	marker := `permission = "RW", bytes = "`
	if !strings.Contains(text, marker) {
		t.Fatalf("generated program text carried no memory backing to alter")
	}
	altered := strings.Replace(text, marker, `permission = "RW", unknown_field = 1, bytes = "`, 1)
	if report := parserRejection(t, altered); report == "" {
		t.Fatal("Isla accepted an unknown nested field, so this check proves nothing")
	}
}

// parserRejection returns the parser's complaint, or an empty string when the
// program is accepted. Only parsing is exercised, not the proof search.
func parserRejection(t *testing.T, text string) string {
	t.Helper()
	parser := requiredPath(t, "HYPERRAY_ARM64_ISLA_DUMP")
	path := filepath.Join(t.TempDir(), "program.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write program: %v", err)
	}
	command := exec.CommandContext(t.Context(), parser,
		"-T", "1",
		"-A", requiredPath(t, "HYPERRAY_ARM64_SAIL_IR"),
		"-C", requiredPath(t, "HYPERRAY_ARM64_ISLA_CONFIG"),
		"-f", "human", path, "--executable-entry", "--initialized-memory")
	output, runError := command.CombinedOutput()
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Failed to parse litmus file") {
			return strings.TrimSpace(strings.Join(parserDetail(output), " "))
		}
	}
	if runError != nil {
		t.Fatalf("%s failed without naming a parse error: %v", parser, runError)
	}
	return ""
}

// parserDetail keeps the parse failure lines and drops the trace dump.
func parserDetail(output []byte) []string {
	detail := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Failed to parse litmus file") {
			continue
		}
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "(") {
			continue
		}
		detail = append(detail, line)
	}
	return detail
}
