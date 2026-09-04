// Trace protocol parsing accepts complete top-level trace expressions only.
// It ignores delimiters inside strings and quoted identifiers.
package isla

func countTraceBlocks(output string) (uint64, error) {
	result, err := scanTraceOutput(output)
	if err != nil {
		return 0, err
	}
	return result.TraceCount, nil
}

func traceStart(value string) bool {
	return len(value) > 6 && value[:6] == "(trace" && asciiSpace(value[6])
}

func asciiSpace(character byte) bool {
	return character == ' ' || character == '\n' || character == '\r' || character == '\t'
}

func traceProtocolError(detail string) error {
	return engineError(ProtocolError, "footprint output", detail)
}
