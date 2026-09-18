// Semantic thread headers use a contiguous zero-based decimal identity.
// A reordered or malformed tree must not be joined to another thread.
package isla

import (
	"strconv"
	"strings"
)

func semanticThreadNumber(line string) (uint64, error) {
	value := strings.TrimSpace(line)
	if !strings.HasPrefix(value, "Thread ") || !strings.HasSuffix(value, ":") {
		return 0, semanticProtocolError("invalid thread header")
	}
	identifier := strings.TrimSuffix(strings.TrimPrefix(value, "Thread "), ":")
	thread, err := strconv.ParseUint(identifier, 10, 64)
	if err != nil {
		return 0, semanticProtocolError("invalid thread number")
	}
	return thread, nil
}
