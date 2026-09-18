// Terminal marker detection recognizes only the record position.
// Herd headers must not be interpreted as terminal evidence.
package isla

import "strings"

func terminalLine(line string) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return false
	}
	if strings.HasPrefix(fields[0], "TerminalEvidence") {
		return true
	}
	return fields[0] != "Test" && len(fields) > 1 && strings.HasPrefix(fields[1], "TerminalEvidence")
}
