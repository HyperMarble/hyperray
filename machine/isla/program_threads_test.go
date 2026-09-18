// Public explicit-thread tests exercise ordered entries through the SDK.
// They require real ELF instruction starts and preserve caller-owned inputs.
package isla_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildProgramWithExplicitThreadsIsDeterministic(t *testing.T) {
	content := machineFixture(t)
	threads := []isla.ThreadEntry{
		{EntryAddress: 0x80100006, InitialRegisters: []isla.RegisterValue{{Name: "x3", Value: "3"}}},
		{EntryAddress: 0x80100000, InitialRegisters: []isla.RegisterValue{{Name: "x4", Value: "4"}}},
	}
	boundary := isla.ProgramBoundary{
		Name: "EXPLICIT-THREADS", Threads: threads, NegatedAssertion: "True", MaximumProgramBytes: 1 << 20,
	}
	first, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	second, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("second BuildProgram() error = %v", err)
	}
	if string(first.Content()) != string(second.Content()) || first.Digest() != second.Digest() {
		t.Fatal("explicit-thread generation is not deterministic")
	}
	source := string(first.Content())
	for _, want := range []string{
		"[thread.0]", "entry = \"0x80100006\"", "x3 = \"3\"",
		"[thread.1]", "entry = \"0x80100000\"", "x4 = \"4\"",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated input lacks %q: %s", want, source)
		}
	}
	evidence := first.Evidence()
	if evidence.ThreadCount != 2 || evidence.ThreadEntries != "0x80100006,0x80100000" {
		t.Errorf("thread evidence = %#v", evidence)
	}
	threads[0].EntryAddress = 0
	threads[0].InitialRegisters[0].Value = "changed"
	if first.Evidence().ThreadEntries != "0x80100006,0x80100000" || strings.Contains(string(first.Content()), "changed") {
		t.Error("program retained caller-owned thread storage")
	}
	entries := first.ThreadEntries()
	entries[0].EntryAddress = 0
	if first.ThreadEntries()[0].EntryAddress != 0x80100006 {
		t.Error("ThreadEntries() returned shared storage")
	}
}

func TestBuildProgramAcceptsOneExplicitThread(t *testing.T) {
	boundary := isla.ProgramBoundary{Name: "ONE-THREAD", Threads: []isla.ThreadEntry{{
		EntryAddress: 0x80100006,
	}}, NegatedAssertion: "True", MaximumProgramBytes: 1 << 20}
	program, err := isla.BuildProgram(machineFixture(t), 1<<20, boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	if program.ThreadCount() != 1 || program.Evidence().ThreadCount != 1 {
		t.Errorf("one explicit thread evidence = %#v", program.Evidence())
	}
}

func TestBuildProgramAllowsRepeatedExplicitEntries(t *testing.T) {
	boundary := isla.ProgramBoundary{
		Name: "REPEATED-THREADS", Threads: []isla.ThreadEntry{
			{EntryAddress: 0x80100000, InitialRegisters: []isla.RegisterValue{{Name: "x2", Value: "1"}}},
			{EntryAddress: 0x80100000, InitialRegisters: []isla.RegisterValue{{Name: "x2", Value: "1"}}},
		}, NegatedAssertion: "True", MaximumProgramBytes: 1 << 20,
	}
	if _, err := isla.BuildProgram(machineFixture(t), 1<<20, boundary); err != nil {
		t.Fatalf("repeated entry was rejected: %v", err)
	}
}

func TestBuildProgramPadsManyThreadNamesForLexicalNativeOrder(t *testing.T) {
	threads := make([]isla.ThreadEntry, 11)
	for index := range threads {
		threads[index] = isla.ThreadEntry{EntryAddress: 0x80100000, InitialRegisters: []isla.RegisterValue{{
			Name: "x2", Value: strconv.Itoa(index),
		}}}
	}
	boundary := isla.ProgramBoundary{Name: "ELEVEN-THREADS", Threads: threads, NegatedAssertion: "True", MaximumProgramBytes: 1 << 20}
	program, err := isla.BuildProgram(machineFixture(t), 1<<20, boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	source := string(program.Content())
	if strings.Contains(source, "[thread.0]\n") || strings.Contains(source, "[thread.1]\n") {
		t.Fatal("many-thread output used unpadded table names")
	}
	for index := range threads {
		name := fmt.Sprintf("[thread.%02d]", index)
		position := strings.Index(source, name)
		if position < 0 {
			t.Errorf("thread %d is missing: %s", index, source)
			continue
		}
		next := strings.Index(source[position+len(name):], "\n[thread.")
		if next < 0 {
			next = len(source) - position - len(name)
		}
		section := source[position : position+len(name)+next]
		if !strings.Contains(section, fmt.Sprintf("x2 = \"%d\"", index)) {
			t.Errorf("thread %d is missing or mismatched: %s", index, source)
		}
	}
}
