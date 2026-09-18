// ARM64 Program validation tests assert stable typed rejection details.
// Each invalid input must return a zero Program before rendering.
package isla_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRejectsCapacityBeforeRendering(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	program, err := isla.BuildARM64Program(content, 32767, arm64ProgramBoundary(start, end))
	assertZeroARM64Program(t, program)
	var rejection *arm64.Rejection
	if !errors.As(err, &rejection) || rejection.Code != arm64.LoadedByteCapacityExceeded || rejection.Detail == "" {
		t.Fatalf("error = %v, want %s with detail", err, arm64.LoadedByteCapacityExceeded)
	}
}

func TestBuildARM64ProgramRejectsUnalignedBounds(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	cases := []struct {
		name     string
		boundary isla.ARM64ProgramBoundary
	}{
		{"start", arm64ProgramBoundary(start+2, end)},
		{"end", arm64ProgramBoundary(start, end-2)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			program, err := isla.BuildARM64Program(content, 32768, testCase.boundary)
			assertZeroARM64Program(t, program)
			assertARM64Error(t, err, isla.InvalidInput, "ARM64 function boundary", "aligned")
		})
	}
}

func TestBuildARM64ProgramRejectsInvalidBoundaryRelations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	cases := []struct {
		name     string
		boundary isla.ARM64ProgramBoundary
		detail   string
	}{
		{"return inside image", arm64ProgramBoundaryWithReturn(start, end, 0x100000300), "inside"},
		{"entry collision", arm64ProgramBoundaryWithReturn(start, end, start), "differ"},
		{"zero return", arm64ProgramBoundaryWithReturn(start, end, 0), "nonzero"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			program, err := isla.BuildARM64Program(content, 32768, testCase.boundary)
			assertZeroARM64Program(t, program)
			assertARM64Error(t, err, isla.InvalidInput, "ARM64 return address", testCase.detail)
		})
	}
}
