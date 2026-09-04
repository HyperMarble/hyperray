// Event-head parsing accepts plain Isla event names only.
// Quoted identifiers remain valid inside each event value.
package isla

func traceEventHead(value string) (string, error) {
	index := 1
	for index < len(value) && asciiSpace(value[index]) {
		index++
	}
	start := index
	for index < len(value) && !asciiSpace(value[index]) && value[index] != ')' {
		if invalidEventHeadByte(value[index]) {
			return "", traceProtocolError("invalid event kind")
		}
		index++
	}
	if start == index {
		return "", traceProtocolError("empty event expression")
	}
	return value[start:index], nil
}

func invalidEventHeadByte(character byte) bool {
	return character == '(' || character == ';' || character == '"' || character == '|'
}
