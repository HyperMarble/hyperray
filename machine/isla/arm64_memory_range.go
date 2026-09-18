// ARM64 range helpers implement checked finite address arithmetic.
// They must never wrap or accept an address above the profile limit.
package isla

func finiteRange(start uint64, length uint64) (uint64, bool) {
	if length == 0 || start >= arm64MemoryLimit || length > arm64MemoryLimit-start {
		return 0, false
	}
	return start + length, true
}

func rangesOverlap(leftStart uint64, leftEnd uint64, rightStart uint64, rightEnd uint64) bool {
	return leftStart < rightEnd && rightStart < leftEnd
}
