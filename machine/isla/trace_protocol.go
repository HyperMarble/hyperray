// Trace protocol parsing accepts complete top-level trace expressions only.
// It ignores delimiters inside strings and quoted identifiers.
package isla

func countTraceBlocks(output string) (uint64, error) {
	scanner := traceScanner{}
	for index := 0; index < len(output); index++ {
		character := output[index]
		if scanner.consumeQuoted(character) {
			continue
		}
		if character == '(' {
			if scanner.depth == 0 && !traceStart(output[index:]) {
				return 0, traceProtocolError("expected trace expression")
			}
			if scanner.depth == 0 {
				scanner.count++
			}
			scanner.depth++
			continue
		}
		if character == ')' {
			if scanner.depth == 0 {
				return 0, traceProtocolError("unexpected closing delimiter")
			}
			scanner.depth--
			continue
		}
		if scanner.depth == 0 && !asciiSpace(character) {
			return 0, traceProtocolError("text outside trace expression")
		}
	}
	if scanner.incomplete() || scanner.count == 0 {
		return 0, traceProtocolError("empty or truncated trace expression")
	}
	return scanner.count, nil
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
