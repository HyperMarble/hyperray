// Semantic parsing separates thread trees, the property, memory, and footprints.
// It accepts a report only when all required sections are unique and ordered.
package isla

import "strings"

func parseSemanticOutput(output string) (semanticSummary, error) {
	lines := strings.Split(output, "\n")
	finalIndex, err := uniqueSemanticMarker(lines, "Final Assertion:")
	if err != nil {
		return semanticSummary{}, err
	}
	memoryIndex, err := uniqueSemanticMarker(lines, "Memory:")
	if err != nil {
		return semanticSummary{}, err
	}
	if finalIndex >= memoryIndex {
		return semanticSummary{}, semanticProtocolError("section order")
	}
	assertion := strings.TrimSpace(strings.Join(lines[finalIndex+1:memoryIndex], "\n"))
	if assertion == "" {
		return semanticSummary{}, semanticProtocolError("empty final assertion")
	}
	threads, err := parseSemanticThreads(lines[:finalIndex])
	if err != nil {
		return semanticSummary{}, err
	}
	footprints, err := parseSemanticFootprints(lines[memoryIndex+1:])
	if err != nil {
		return semanticSummary{}, err
	}
	if !equalEncodingSets(threads.encodings, footprints) {
		return semanticSummary{}, engineError(CoverageMismatch, "semantic instructions", "instruction and footprint sets differ")
	}
	return threads.summary(assertion, footprints), nil
}

func uniqueSemanticMarker(lines []string, prefix string) (int, error) {
	found := -1
	for index := range lines {
		if !strings.HasPrefix(strings.TrimSpace(lines[index]), prefix) {
			continue
		}
		if found >= 0 {
			return 0, semanticProtocolError("duplicate " + prefix)
		}
		found = index
	}
	if found < 0 {
		return 0, semanticProtocolError("missing " + prefix)
	}
	return found, nil
}

func semanticProtocolError(detail string) error {
	return engineError(ProtocolError, "semantic output", detail)
}
