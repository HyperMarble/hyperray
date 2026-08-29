package tests

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The freeze file is only worth having if something checks it. It listed a
// stale digest for finalarchitecture.md through an entire session of work --
// the document had been revised and the digest never re-stamped, so the freeze
// silently certified bytes that no longer existed. Nothing failed, because
// nothing looked.
//
// A frozen document that drifts unnoticed is worse than an unfrozen one: it
// reads as authoritative while describing something else.

func TestArchitectureFreezeMatchesDocuments(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "docs", "specs")
	freezePath := filepath.Join(dir, "architecture-freeze.sha256")

	file, err := os.Open(freezePath)
	if err != nil {
		t.Fatalf("opening the freeze: %v", err)
	}
	defer file.Close()

	listed := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// `shasum -a 256` writes "<hex>  <path>", two spaces.
		digest, name, found := strings.Cut(line, "  ")
		if !found {
			t.Errorf("unparsable freeze line %q", line)
			continue
		}
		listed++

		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("frozen document %q: %v", name, err)
			continue
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != digest {
			t.Errorf("%s drifted from its freeze:\n  frozen %s\n  actual %s\nre-stamp it deliberately, or restore the document", name, digest, got)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading the freeze: %v", err)
	}
	if listed == 0 {
		t.Fatal("the freeze lists no documents")
	}
}

// The evidence rule decides where every spec row comes from, so both authoring
// skills have to send the author to it rather than restating it and drifting.
func TestSkillsCiteTheEvidenceRule(t *testing.T) {
	for _, skill := range []string{"skills/spec/SKILL.md", "skills/task/SKILL.md"} {
		body := readRepoFile(t, skill)
		if !strings.Contains(body, "evidence-rule.md") {
			t.Errorf("%s does not link docs/specs/evidence-rule.md", skill)
		}
		// The rule the two skills previously contradicted: the reference was
		// forbidden from adding anything the prompt had not already stated.
		if strings.Contains(body, "cannot silently add a requirement absent from the") {
			t.Errorf("%s still carries the superseded instruction-only rule", skill)
		}
	}
}

// The scaffold embedded in the binary must stay byte-identical to the real
// template and schema, or `hyperray spec-init` hands authors a stale format.
func TestScaffoldMatchesSkill(t *testing.T) {
	for embedded, original := range map[string]string{
		"cmd/hyperray/scaffold/spec.md":   "skills/spec/templates/spec.md",
		"cmd/hyperray/scaffold/schema.md": "skills/spec/references/schema.md",
	} {
		if readRepoFile(t, embedded) != readRepoFile(t, original) {
			t.Errorf("%s drifted from %s; re-copy it", embedded, original)
		}
	}
}
