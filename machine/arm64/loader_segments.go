// This file collects validated Mach-O segments for mapping.
// It must not materialize bytes or admit PAGEZERO.
package arm64

import (
	"debug/macho"
	"sort"

	"github.com/HyperMarble/hyperray/machine"
)

type mappedSegment struct {
	header      macho.SegmentHeader
	permissions machine.Permissions
}

func collectSegments(content []byte, maximum uint64) ([]mappedSegment, error) {
	headers, err := ReadSegmentHeaders(content)
	if err != nil {
		return nil, err
	}
	segments, err := validateSegmentHeaders(headers, maximum, content)
	if err != nil {
		return nil, err
	}
	sort.Slice(segments, func(left, right int) bool { return segments[left].header.Addr < segments[right].header.Addr })
	return segments, nil
}

func validateSegmentHeaders(headers []macho.SegmentHeader, maximum uint64, content []byte) ([]mappedSegment, error) {
	segments := make([]mappedSegment, 0, len(headers))
	for index, header := range headers {
		permissions, err := SegmentPermissions(header)
		if err != nil {
			return nil, err
		}
		if err := ValidateSegmentFlags(header); err != nil {
			return nil, err
		}
		if err := validateSpecialSegment(header, index); err != nil {
			return nil, err
		}
		if header.Name != "__PAGEZERO" {
			segments = append(segments, mappedSegment{header: header, permissions: permissions})
		}
	}
	if err := validateSegmentLayout(headers); err != nil {
		return nil, err
	}
	if err := validateMaterializedSize(segments, maximum); err != nil {
		return nil, err
	}
	if err := validateTextHeader(headers, content); err != nil {
		return nil, err
	}
	return segments, nil
}
