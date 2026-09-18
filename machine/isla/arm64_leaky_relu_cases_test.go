//go:build isla_integration && arm64_acceptance

// Leaky case helpers bind the checked-in six-row manifest to public metadata.
// They must not rebuild or modify the genuine fixture source.
package isla_test

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

type leakyCase struct {
	Name          string   `json:"name"`
	InputWords    []string `json:"input_words"`
	ExpectedWords []string `json:"expected_words"`
	NegativeCount int      `json:"negative_count"`
}

func leakyCases(t *testing.T) []leakyCase {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "machine", "arm64", "leaky_relu", "cases.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Leaky cases: %v", err)
	}
	var cases []leakyCase
	if err := json.Unmarshal(content, &cases); err != nil {
		t.Fatalf("decode Leaky cases: %v", err)
	}
	if len(cases) != 6 {
		t.Fatalf("Leaky cases count = %d, want 6", len(cases))
	}
	return cases
}

func leakyMemoryInput(t *testing.T, words []string) isla.ARM64MemoryInput {
	t.Helper()
	input := make([]byte, len(words)*4)
	for index, word := range words {
		value, err := strconv.ParseUint(word, 0, 32)
		if err != nil {
			t.Fatalf("Leaky input word %q: %v", word, err)
		}
		binary.LittleEndian.PutUint32(input[index*4:], uint32(value))
	}
	memory, err := isla.NewARM64MemoryInput(isla.ARM64NormalS1Fixed4KSIMD, isla.TableArena{Base: 0x5000, CapacityPages: 41}, leakyMappings(), []isla.MemoryBacking{
		{Address: 0x400000, Permission: isla.MemoryReadWrite, Bytes: input},
		{Address: 0x3bf0, Permission: isla.MemoryReadWrite, Bytes: make([]byte, 80)},
	})
	if err != nil {
		t.Fatalf("NewARM64MemoryInput() error = %v", err)
	}
	return memory
}

func leakyObservations() []isla.MemoryObservation {
	observations := make([]isla.MemoryObservation, 8)
	for index := range observations {
		observations[index] = isla.MemoryObservation{Name: "lane" + strconv.Itoa(index), Address: 0x400000 + uint64(index*4), Bytes: 4}
	}
	return observations
}
