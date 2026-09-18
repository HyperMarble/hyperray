// Semantic summaries expose the validated counts and both compared sets.
// Sorted encodings make evidence stable across tool scheduling orders.
package isla

func (threads semanticThreads) summary(assertion string, footprints map[string]struct{}) semanticSummary {
	return semanticSummary{
		threadCount: threads.count, traceCount: threads.traceCount,
		instructionEventCount: threads.eventCount,
		instructionEncodings:  sortedEncodings(threads.encodings),
		instructions:          sortedSemanticInstructions(threads.instructions),
		threads:               copySemanticThreads(threads.records),
		footprintEncodings:    sortedEncodings(footprints), finalAssertion: assertion,
	}
}

func copySemanticThreads(values []SemanticThread) []SemanticThread {
	result := make([]SemanticThread, len(values))
	for index := range values {
		result[index] = values[index]
		result[index].EntryAddresses = append([]uint64(nil), values[index].EntryAddresses...)
		result[index].Instructions = append([]SemanticInstruction(nil), values[index].Instructions...)
	}
	return result
}
