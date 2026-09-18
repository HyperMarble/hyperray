// A negative-only result must retain one forbidden row per candidate.
// Missing states or unexpected concrete witnesses cannot yield a proof result.
package isla

import "strings"

func forbiddenRows(rows []string) error {
	for _, row := range rows {
		value := strings.TrimSpace(row)
		if value != "???;" && value != "forbidden ???;" {
			return engineError(ProtocolError, "states", "negative result contains a non-forbidden row")
		}
	}
	return nil
}
