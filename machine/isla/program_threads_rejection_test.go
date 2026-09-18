// Explicit-thread rejection tests start from an accepted public input.
// Each case changes one property and requires its declared error.
package isla_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildProgramRejectsExplicitThreadInputs(t *testing.T) {
	cases := []struct {
		name            string
		change          func(*isla.ProgramBoundary)
		code            isla.ErrorCode
		subject, detail string
	}{
		{"zero", func(boundary *isla.ProgramBoundary) { boundary.Threads = []isla.ThreadEntry{} }, isla.InvalidInput, "threads", "explicit set is empty"},
		{"entry", func(boundary *isla.ProgramBoundary) { boundary.Threads[0].EntryAddress = 0x80100002 }, isla.CoverageMismatch, "thread entry", "is not an instruction start"},
		{"legacy-address", func(boundary *isla.ProgramBoundary) { boundary.ThreadAddress = 0x80100000 }, isla.InvalidInput, "thread boundary", "legacy entry and registers conflict with explicit threads"},
		{"legacy-registers", func(boundary *isla.ProgramBoundary) {
			boundary.InitialRegisters = []isla.RegisterValue{{Name: "x1", Value: "1"}}
		}, isla.InvalidInput, "thread boundary", "legacy entry and registers conflict with explicit threads"},
		{"duplicate-register", func(boundary *isla.ProgramBoundary) {
			boundary.Threads[0].InitialRegisters = append(boundary.Threads[0].InitialRegisters, isla.RegisterValue{Name: "x2", Value: "2"})
		}, isla.InvalidInput, "register name", "duplicate x2"},
		{"sequential", func(boundary *isla.ProgramBoundary) { boundary.MemoryProfile = isla.SequentialMemory }, isla.InvalidInput, "threads", "sequential memory supports one thread"},
		{"output-limit", func(boundary *isla.ProgramBoundary) { boundary.MaximumProgramBytes = 1 }, isla.ResourceLimit, "generated program", "program size limit reached"},
	}
	content := machineFixture(t)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			boundary := acceptedExplicitBoundary(t, content)
			testCase.change(&boundary)
			program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
			var failure *isla.Error
			if program.Digest() != "" || !errors.As(err, &failure) || failure.Code != testCase.code || failure.Subject != testCase.subject || failure.Detail != testCase.detail {
				t.Fatalf("BuildProgram() = %q, %v; want %s: %s: %s", program.Digest(), err, testCase.code, testCase.subject, testCase.detail)
			}
		})
	}
}

func TestBuildProgramRejectsUnsupportedTypedStateForMultipleThreads(t *testing.T) {
	content := machineFixture(t)
	boundary := acceptedExplicitBoundary(t, content)
	boundary.InitialState = []isla.RegisterValue{{Name: "mcause", Value: "0"}}
	program, err := isla.BuildProgram(content, uint64(len(content)), boundary)
	var failure *isla.Error
	if program.Digest() != "" || !errors.As(err, &failure) || failure.Code != isla.InvalidInput || failure.Subject != "initial state" || failure.Detail != "typed state is unsupported for multiple threads" {
		t.Fatalf("typed state error = %v, want unsupported multi-thread state", err)
	}
}

func acceptedExplicitBoundary(t *testing.T, content []byte) isla.ProgramBoundary {
	t.Helper()
	boundary := isla.ProgramBoundary{
		Name: "THREAD-BASELINE", NegatedAssertion: "True", MaximumProgramBytes: 1 << 20,
		Threads: []isla.ThreadEntry{
			{EntryAddress: 0x80100000, InitialRegisters: []isla.RegisterValue{{Name: "x2", Value: "1"}}},
			{EntryAddress: 0x80100000},
		},
	}
	if _, err := isla.BuildProgram(content, uint64(len(content)), boundary); err != nil {
		t.Fatalf("valid explicit-thread baseline rejected: %v", err)
	}
	return boundary
}
