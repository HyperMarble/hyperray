// This file exposes parsed Mach-O segments to independent loader test oracles.
package arm64_test

import "debug/macho"

func fixtureSegments(file *macho.File) []*macho.Segment {
	segments := make([]*macho.Segment, 0)
	for _, load := range file.Loads {
		if segment, ok := load.(*macho.Segment); ok {
			segments = append(segments, segment)
		}
	}
	return segments
}
