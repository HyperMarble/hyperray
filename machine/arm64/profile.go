// This file defines the ARM64 Mach-O loader identity and boundary input.
// It must not add symbol or execution semantics.
package arm64

// ProfileName identifies the static little-endian ARM64 Mach-O profile.
const ProfileName = "static-little-endian-arm64-macos-macho"

// FunctionBoundary declares the half-open code range for analysis.
type FunctionBoundary struct {
	StartAddress uint64
	EndAddress   uint64
}
