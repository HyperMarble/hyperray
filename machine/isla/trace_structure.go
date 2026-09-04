// Trace structure recognizes wrappers and direct event heads only.
// Nested value expressions remain uninterpreted data for later stages.
package isla

func (parser *traceParser) openExpression(value string) (int, error) {
	if parser.scanner.depth == 0 {
		return parser.openTrace(value)
	}
	if parser.scanner.depth == 1 {
		if err := parser.recordEvent(value); err != nil {
			return 0, err
		}
	}
	parser.scanner.depth++
	return 0, nil
}

func (parser *traceParser) openTrace(value string) (int, error) {
	if !traceStart(value) {
		return 0, traceProtocolError("expected trace expression")
	}
	parser.scanner.depth = 1
	parser.currentEvents = 0
	parser.result.TraceCount++
	return len("(trace") - 1, nil
}

func (parser *traceParser) recordEvent(value string) error {
	kind, err := traceEventHead(value)
	if err != nil {
		return err
	}
	parser.result.Events = append(parser.result.Events, scannedTraceEvent{
		TraceIndex: parser.result.TraceCount - 1,
		EventIndex: parser.currentEvents,
		Kind:       kind,
	})
	parser.currentEvents++
	return nil
}

func (parser *traceParser) closeExpression() error {
	if parser.scanner.depth == 0 {
		return traceProtocolError("unexpected closing delimiter")
	}
	parser.scanner.depth--
	if parser.scanner.depth == 0 && parser.currentEvents == 0 {
		return traceProtocolError("empty trace expression")
	}
	return nil
}

func (parser traceParser) acceptText(character byte) error {
	if parser.scanner.depth <= 1 && !asciiSpace(character) {
		return traceProtocolError("text outside event expression")
	}
	return nil
}
