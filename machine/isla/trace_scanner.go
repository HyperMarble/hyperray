// Trace scanner state separates quoted text from structural delimiters.
package isla

type traceScanner struct {
	depth   uint64
	count   uint64
	comment bool
	quoted  bool
	symbol  bool
	escaped bool
}

func (scanner *traceScanner) consumeQuoted(character byte) bool {
	if scanner.comment {
		scanner.comment = character != '\n'
		return true
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
	scanner.comment = character == ';'
	scanner.quoted = character == '"'
	scanner.symbol = character == '|'
	return scanner.comment || scanner.quoted || scanner.symbol
}

func (scanner traceScanner) incomplete() bool {
	return scanner.depth != 0 || scanner.quoted || scanner.symbol
}
