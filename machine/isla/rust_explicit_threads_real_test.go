//go:build isla_integration

// The native test compiles one ELF and starts two explicit machine threads.
// It measures thread ownership through the public executable SDK.
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

func TestRealRustExplicitThreads(t *testing.T) {
	content := compileRustExecutable(t, "branch.rs")
	boundary := explicitRustThreadsBoundary(t, content, "~(0:x10 = 17 & 1:x10 = 8)")
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "explicit-threads.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request := explicitThreadsRequest(t, path)
	result, err := realExecutableVerifier(t).VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes,
	})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	if result.Verification.Status != isla.Proved || len(result.Execution.Threads) != 2 {
		t.Fatalf("two-thread proof = %#v", result)
	}
	changed := boundary
	changed.NegatedAssertion = "~(0:x10 = 18 & 1:x10 = 8)"
	changedProgram, err := isla.BuildProgram(content, uint64(len(content)), changed)
	if err != nil {
		t.Fatalf("changed BuildProgram() error = %v", err)
	}
	changedPath := filepath.Join(t.TempDir(), "changed-threads.toml")
	if err := os.WriteFile(changedPath, changedProgram.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	changedResult, err := realExecutableVerifier(t).VerifyProgram(t.Context(), explicitThreadsRequest(t, changedPath), changedProgram, isla.ExecutableLimits{
		ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes,
	})
	if err != nil || changedResult.Verification.Status != isla.Disproved || !hasBothThreadValues(changedResult.Verification.CounterexampleState) {
		t.Fatalf("changed two-thread requirement = %#v, %v", changedResult, err)
	}
	changed.NegatedAssertion = "~(0:x10 = 17 & 1:x10 = 9)"
	changedProgram, err = isla.BuildProgram(content, uint64(len(content)), changed)
	if err != nil {
		t.Fatalf("second changed BuildProgram() error = %v", err)
	}
	changedPath = filepath.Join(t.TempDir(), "changed-thread-one.toml")
	if err := os.WriteFile(changedPath, changedProgram.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	threadOneResult, err := realExecutableVerifier(t).VerifyProgram(t.Context(), explicitThreadsRequest(t, changedPath), changedProgram, isla.ExecutableLimits{
		ThreadLimit: 2, TimeLimitSeconds: 60, MaximumOutputBytes: realELFOutputLimitBytes,
	})
	if err != nil || threadOneResult.Verification.Status != isla.Disproved || !hasBothThreadValues(threadOneResult.Verification.CounterexampleState) {
		t.Fatalf("changed thread-one requirement = %#v, %v", threadOneResult, err)
	}
}

func TestRealRustExplicitThreadOrdering(t *testing.T) {
	content := compileRustExecutable(t, "branch.rs")
	image, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := image.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	var returnAddress uint64
	for _, symbol := range symbols {
		if symbol.Name == "__hyperray_return" {
			returnAddress = symbol.Value
		}
	}
	if returnAddress == 0 {
		t.Fatal("compiler-built ELF has no declared return symbol")
	}
	threads := make([]isla.ThreadEntry, 11)
	for index := range threads {
		threads[index] = threadEntry(image.Entry, returnAddress, uint64(index))
	}
	boundary := isla.ProgramBoundary{
		Name: "rust-explicit-thread-ordering", Threads: threads,
		NegatedAssertion: "~(2:x10 = 32 & 10:x10 = 6)", MaximumProgramBytes: 1 << 20,
	}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	if err != nil {
		t.Fatalf("BuildProgram() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "eleven-threads.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	request, err := isla.NewVerificationRequest(realRequestPathWithLimits(t, path, 2), 1, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	result, err := realExecutableVerifier(t).VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 1, TimeLimitSeconds: 180, MaximumOutputBytes: realELFOutputLimitBytes,
	})
	if err != nil {
		t.Fatalf("VerifyProgram() error = %v", err)
	}
	if result.Verification.Status != isla.Proved || result.Program.ThreadCount != 11 || len(result.Execution.Threads) != 11 {
		t.Fatalf("eleven-thread proof = %#v", result.Verification)
	}
	if result.Footprints.Evidence.ThreadLimit != 1 || result.Verification.Semantics.Evidence.ThreadLimit != 1 || result.Verification.Semantics.Evidence.PCVisitLimit != 2 {
		t.Fatalf("independent limits were not retained: %#v", result.Verification.Semantics.Evidence)
	}
	state := result.Verification.Semantics.TraceOutput
	for _, id := range []uint64{2, 10} {
		thread, ok := threadByID(result.Execution.Threads, id)
		if !ok || thread.EntryAddress != image.Entry || len(thread.Instructions) == 0 {
			t.Fatalf("native output omitted independent thread %d: %#v", id, result.Execution.Threads)
		}
		if _, ok := traceThread(state, id); !ok {
			t.Fatalf("native trace omitted thread %d: %s", id, state)
		}
	}
}

func threadByID(threads []isla.SemanticThread, id uint64) (isla.SemanticThread, bool) {
	for _, thread := range threads {
		if thread.ID == id {
			return thread, true
		}
	}
	return isla.SemanticThread{}, false
}

func explicitThreadsRequest(t *testing.T, path string) isla.VerificationRequest {
	t.Helper()
	request, err := isla.NewVerificationRequest(realRequestPath(t, path), 2, 2048)
	if err != nil {
		t.Fatalf("NewVerificationRequest() error = %v", err)
	}
	return request
}

func hasBothThreadValues(state string) bool {
	return strings.Contains(state, "0:x10=#x0000000000000011;") &&
		strings.Contains(state, "1:x10=#x0000000000000008;")
}

func explicitRustThreadsBoundary(t *testing.T, content []byte, assertion string) isla.ProgramBoundary {
	t.Helper()
	image, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := image.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols {
		if symbol.Name == "__hyperray_return" {
			return isla.ProgramBoundary{
				Name: "rust-explicit-threads", Threads: []isla.ThreadEntry{
					threadEntry(image.Entry, symbol.Value, 0), threadEntry(image.Entry, symbol.Value, 8),
				}, NegatedAssertion: assertion, MaximumProgramBytes: 1 << 20,
			}
		}
	}
	t.Fatal("compiler-built ELF has no declared return symbol")
	return isla.ProgramBoundary{}
}

func threadEntry(entry uint64, returnAddress uint64, input uint64) isla.ThreadEntry {
	return isla.ThreadEntry{EntryAddress: entry, InitialRegisters: []isla.RegisterValue{
		{Name: "x1", Value: fmt.Sprintf("%#x", returnAddress)}, {Name: "x10", Value: fmt.Sprintf("%d", input)},
	}}
}
