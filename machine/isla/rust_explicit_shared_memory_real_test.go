//go:build isla_integration

// This fixture measures one ELF-backed atomic location shared by two explicit
// machine threads. It does not model Rust thread creation or a scheduler.
package isla_test

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustExplicitSharedAtomic(t *testing.T) {
	content := compileRustExecutable(t, "atomic_shared.rs")
	writer, reader, shared, returnAddress := atomicSymbols(t, content)
	threads := []isla.ThreadEntry{
		{EntryAddress: writer, InitialRegisters: []isla.RegisterValue{{Name: "x1", Value: fmt.Sprintf("%#x", returnAddress)}}},
		{EntryAddress: reader, InitialRegisters: []isla.RegisterValue{{Name: "x1", Value: fmt.Sprintf("%#x", returnAddress)}}},
	}

	boundary := isla.ProgramBoundary{
		Name: "rust-shared-atomic", Threads: threads,
		NegatedAssertion: "~(1:x10 = 0 | 1:x10 = 1)", MaximumProgramBytes: 1 << 20,
	}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	if !strings.Contains(string(program.Content()), fmt.Sprintf("address = \"0x%x\"", shared)) {
		t.Fatalf("generated program omitted shared ELF address %#x", shared)
	}
	result := runAtomicQuery(t, program, "atomic-set.toml")
	if result.Verification.Status != isla.Proved || len(result.Execution.Threads) != 2 {
		t.Fatalf("atomic output-set proof = %#v", result.Verification)
	}
	traceAddress := fmt.Sprintf("%x", shared)
	trace := result.Verification.Semantics.TraceOutput
	writerTrace, writerOK := traceThread(trace, 0)
	readerTrace, readerOK := traceThread(trace, 1)
	writerEvents := traceMemoryEvents(writerTrace, "write-mem", traceAddress)
	readerEvents := traceMemoryEvents(readerTrace, "read-mem", traceAddress)
	t.Logf("shared ELF address=%#x writer-events=%q reader-events=%q", shared, writerEvents, readerEvents)
	if !writerOK || !readerOK || len(writerEvents) == 0 || len(readerEvents) == 0 {
		t.Fatalf("atomic trace does not show writer write and reader read at %#x", shared)
	}

	for _, value := range []uint64{0, 1} {
		query := boundary
		query.NegatedAssertion = fmt.Sprintf("1:x10 = %d", value)
		queryProgram, err := isla.BuildProgram(content, uint64(len(content)), query)
		if err != nil {
			t.Fatalf("reachable-%d BuildProgram() error = %v", value, err)
		}
		reachable := runAtomicQuery(t, queryProgram, fmt.Sprintf("atomic-reachable-%d.toml", value))
		if reachable.Verification.Status != isla.Disproved || !strings.Contains(reachable.Verification.CounterexampleState, fmt.Sprintf("1:x10=#x%016x;", value)) {
			t.Fatalf("reader value %d was not independently reachable: %#v", value, reachable.Verification)
		}
	}
}

func traceThread(trace string, id uint64) (string, bool) {
	marker := fmt.Sprintf("Thread %d:", id)
	start := strings.Index(trace, marker)
	if start < 0 {
		return "", false
	}
	start += len(marker)
	end := len(trace)
	if next := strings.Index(trace[start:], "\nThread "); next >= 0 {
		end = start + next
	}
	if final := strings.Index(trace[start:end], "\nFinal Assertion:"); final >= 0 {
		end = start + final
	}
	return trace[start:end], true
}

func traceMemoryEvents(section, kind, address string) []string {
	want := "#x" + strings.Repeat("0", 16-len(address)) + address
	var events []string
	for _, line := range strings.Split(section, "\n") {
		fields, ok := traceMemoryFields(line, kind)
		if !ok || len(fields) < 3 {
			continue
		}
		if strings.EqualFold(fields[2], want) {
			events = append(events, line)
		}
	}
	return events
}

func traceMemoryFields(line, kind string) ([]string, bool) {
	text := strings.TrimSpace(line)
	prefix := "(" + kind
	if !strings.HasPrefix(text, prefix) || len(text) == len(prefix) || text[len(prefix)] != ' ' {
		return nil, false
	}
	arguments := strings.TrimSuffix(strings.TrimSpace(text[len(prefix):]), ")")
	return traceEventFields(arguments), true
}

func traceEventFields(text string) []string {
	var fields []string
	for strings.TrimSpace(text) != "" {
		field, remainder, ok := nextTraceEventField(text)
		if !ok {
			return nil
		}
		fields = append(fields, field)
		text = remainder
	}
	return fields
}

func nextTraceEventField(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	start := 0
	depth := 0
	quoted := false
	escaped := false
	for index := start; index < len(text); index++ {
		character := text[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quoted {
			escaped = true
			continue
		}
		if character == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if character == '(' {
			depth++
		}
		if character == ')' {
			depth--
		}
		if character == ' ' && depth == 0 {
			return strings.TrimRight(text[start:index], ")"), text[index+1:], true
		}
	}
	if depth < 0 || quoted {
		return "", "", false
	}
	return strings.TrimRight(text[start:], ")"), "", true
}

func TestThreadTraceMemoryOwnershipRejectsConcentratedEvents(t *testing.T) {
	address := "0000000080101000"
	baseline := "Thread 0:\n(write-mem v0 |WriteKind| #x" + address + " #x01 8)\n(read-mem #x" + address + " |ReadKind| #x" + address + " 8)\nThread 1:\n(write-mem v1 |WriteKind| #x" + address + " #x01 8)\n(read-mem #x" + address + " |ReadKind| #x" + address + " 8)\nFinal Assertion:\nTrue"
	for _, id := range []uint64{0, 1} {
		section, ok := traceThread(baseline, id)
		if !ok {
			t.Fatalf("valid baseline omitted thread %d", id)
		}
		if writes := len(traceMemoryEvents(section, "write-mem", address)); writes != 1 {
			t.Fatalf("valid baseline thread %d write count = %d, want 1", id, writes)
		}
		if reads := len(traceMemoryEvents(section, "read-mem", address)); reads != 1 {
			t.Fatalf("valid baseline thread %d read count = %d, want 1", id, reads)
		}
	}
	mutated := strings.Replace(baseline,
		"Thread 1:\n(write-mem v1 |WriteKind| #x"+address+" #x01 8)\n(read-mem #x"+address+" |ReadKind| #x"+address+" 8)",
		"Thread 1:\n(write-mem v1 |WriteKind| #x"+address+" #x01 8)\n(read-mem #x"+address+" |ReadKind| #x0000000080102000 8)", 1)
	if mutated == baseline {
		t.Fatal("address mutation did not change the accepted baseline")
	}
	reader, readerOK := traceThread(mutated, 1)
	if !readerOK || len(traceMemoryEvents(reader, "read-mem", address)) != 0 {
		t.Fatal("address mutation incorrectly matched a read at the shared address")
	}
}

func runAtomicQuery(t *testing.T, program isla.Program, name string) isla.ExecutableResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request, err := isla.NewVerificationRequest(realRequestPath(t, path), 2, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	result, err := realExecutableVerifier(t).VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 2, TimeLimitSeconds: 120, MaximumOutputBytes: realELFOutputLimitBytes,
	})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	return result
}

func atomicSymbols(t *testing.T, content []byte) (uint64, uint64, uint64, uint64) {
	t.Helper()
	image, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := image.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	var writer, reader, shared, returnAddress uint64
	for _, symbol := range symbols {
		switch symbol.Name {
		case "writer":
			writer = symbol.Value
		case "reader":
			reader = symbol.Value
		case "SHARED":
			shared = symbol.Value
		case "__hyperray_return":
			returnAddress = symbol.Value
		}
	}
	if writer == 0 || reader == 0 || shared == 0 || returnAddress == 0 {
		t.Fatalf("atomic ELF symbols writer=%#x reader=%#x shared=%#x return=%#x", writer, reader, shared, returnAddress)
	}
	return writer, reader, shared, returnAddress
}
