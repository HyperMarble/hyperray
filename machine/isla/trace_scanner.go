// Trace scanner state separates quoted text from structural delimiters.
package isla

type traceScanner struct {
	depth      uint64
	annotation bool
	quoted     bool
	symbol     bool
	escaped    bool
}

func (scanner *traceScanner) consumeLiteral(character byte, remainder string) bool {
	if scanner.annotation {
		return scanner.consumeAnnotation(character, remainder)
	}
	if scanner.quoted {
		wasEscaped := scanner.escaped
		scanner.escaped = character == '\\' && !wasEscaped
		scanner.quoted = character != '"' || wasEscaped
		return true
	}
	if scanner.symbol {
		scanner.symbol = character != '|'
		return true
	}
	scanner.annotation = character == ';' && scanner.depth == 1
	scanner.quoted = character == '"'
	scanner.symbol = character == '|'
	return scanner.annotation || scanner.quoted || scanner.symbol
}

func (scanner *traceScanner) consumeAnnotation(character byte, remainder string) bool {
	if character == '\n' {
		scanner.annotation = false
		return true
	}
	if character == ')' && scanner.depth == 1 && finalLineClose(remainder) {
		scanner.annotation = false
		return false
	}
	return true
}

func finalLineClose(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] == '\n' {
			return nextTraceOrEnd(value[index+1:])
		}
		if value[index] == ')' {
			return false
		}
	}
	return true
}

func nextTraceOrEnd(value string) bool {
	index := 0
	for index < len(value) && asciiSpace(value[index]) {
		index++
	}
	return index == len(value) || traceStart(value[index:])
}

func (scanner traceScanner) incomplete() bool {
	return scanner.depth != 0 || scanner.annotation || scanner.quoted || scanner.symbol
}
