// Trace scanning records each direct event without interpreting its meaning.
// It must reject text that the pinned Isla trace writer does not frame.
package isla

type scannedTraceEvent struct {
	TraceIndex uint64
	EventIndex uint64
	Kind       string
}

type traceScan struct {
	TraceCount uint64
	Events     []scannedTraceEvent
}

type traceParser struct {
	scanner       traceScanner
	result        traceScan
	currentEvents uint64
}

func scanTraceOutput(output string) (traceScan, error) {
	parser := traceParser{}
	for index := 0; index < len(output); {
		advance, err := parser.consumeCharacter(output, index)
		if err != nil {
			return traceScan{}, err
		}
		index += advance + 1
	}
	return parser.finish()
}

func (parser *traceParser) consumeCharacter(output string, index int) (int, error) {
	character := output[index]
	if parser.scanner.consumeLiteral(character, output[index+1:]) {
		return 0, nil
	}
	switch character {
	case '(':
		return parser.openExpression(output[index:])
	case ')':
		return 0, parser.closeExpression()
	default:
		return 0, parser.acceptText(character)
	}
}

func (parser traceParser) finish() (traceScan, error) {
	if parser.scanner.incomplete() || parser.result.TraceCount == 0 {
		return traceScan{}, traceProtocolError("empty or truncated trace expression")
	}
	return parser.result, nil
}
