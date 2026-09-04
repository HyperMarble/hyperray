// Trace protocol parsing accepts complete top-level trace blocks only.
// It rejects empty and truncated output before coverage receives evidence.
package isla

import "strings"

func countTraceBlocks(output string) (uint64, error) {
	remaining := strings.TrimSpace(output)
	if remaining == "" {
		return 0, engineError(ProtocolError, "footprint output", "empty")
	}
	var count uint64
	for remaining != "" {
		if !strings.HasPrefix(remaining, "(trace\n") {
			return 0, engineError(ProtocolError, "footprint output", "expected trace block")
		}
		end := strings.Index(remaining, "\n)\n")
		if end >= 0 {
			count++
			remaining = strings.TrimSpace(remaining[end+3:])
			continue
		}
		if strings.HasSuffix(remaining, "\n)") {
			count++
			remaining = ""
			continue
		}
		return 0, engineError(ProtocolError, "footprint output", "truncated trace block")
	}
	return count, nil
}
