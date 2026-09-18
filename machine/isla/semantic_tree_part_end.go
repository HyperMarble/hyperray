// Part boundaries reuse the trace scanner's quoted-literal rules.
// Unbalanced expressions and unfinished literals return protocol errors.
package isla

func semanticTreePartEnd(value string, start int) (int, error) {
	scanner := traceScanner{}
	for index := start; index < len(value); index++ {
		character := value[index]
		if !scanner.quoted && !scanner.symbol && character == ';' {
			index = semanticTreeCommentEnd(value, index) - 1
			continue
		}
		literal := scanner.consumeLiteral(character, value[index+1:])
		if literal && scanner.depth == 0 && !scanner.quoted && !scanner.symbol {
			return index + 1, nil
		}
		if literal {
			continue
		}
		if scanner.depth == 0 && asciiSpace(character) {
			return index, nil
		}
		done, err := semanticTreeDelimiter(&scanner, character)
		if err != nil {
			return 0, err
		}
		if done {
			return index + 1, nil
		}
	}
	if scanner.incomplete() {
		return 0, semanticProtocolError("unfinished tree part")
	}
	return len(value), nil
}

func semanticTreeDelimiter(scanner *traceScanner, character byte) (bool, error) {
	switch character {
	case '(':
		scanner.depth++
	case ')':
		if scanner.depth == 0 {
			return false, semanticProtocolError("unexpected tree closing delimiter")
		}
		scanner.depth--
		return scanner.depth == 0, nil
	default:
		return false, nil
	}
	return false, nil
}
