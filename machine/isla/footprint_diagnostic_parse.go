// Diagnostic parsing accepts only the upstream missing-primitive warning.
// Any other diagnostic stops the instruction operation.
package isla

import "strings"

const missingPrimitivePrefix = "No primop "

func classifyDiagnostics(output commandOutput) ([]DiagnosticDisposition, error) {
	lines := diagnosticLines(output.diagnostics)
	result := make([]DiagnosticDisposition, 0, len(lines))
	digest := rawOutputDigest(output)
	for index := range lines {
		line := lines[index]
		if !strings.HasPrefix(line, missingPrimitivePrefix) {
			return nil, engineError(ProtocolError, "footprint diagnostic", line)
		}
		result = append(result, DiagnosticDisposition{
			Message: line, Kind: UnavailablePrimitive,
			Disposition: NotCalledInCompletedExecution, EvidenceDigest: digest,
		})
	}
	return result, nil
}

func diagnosticLines(diagnostics string) []string {
	values := strings.Split(diagnostics, "\n")
	result := make([]string, 0, len(values))
	for index := range values {
		line := strings.TrimSpace(values[index])
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
