// This file joins adjacent pure-code regions for boundary validation.
// It must not join regions separated by padding, data, or headers.
package arm64

import "github.com/HyperMarble/hyperray/machine"

func mergeCodeRegions(regions []machine.ExecutableRegion) []machine.ExecutableRegion {
	merged := make([]machine.ExecutableRegion, 0, len(regions))
	for _, region := range regions {
		if len(merged) == 0 {
			merged = append(merged, region)
			continue
		}
		last := &merged[len(merged)-1]
		if last.StartAddress+last.ByteLength != region.StartAddress {
			merged = append(merged, region)
			continue
		}
		last.ByteLength += region.ByteLength
	}
	return merged
}
