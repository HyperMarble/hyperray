// Protocol decimal fields use canonical unsigned decimal syntax.
// Signs, leading zeroes, and non-digit bytes are not accepted.
package isla

import (
	"strconv"
	"strings"
)

func canonicalDecimal(value string) (uint64, bool) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func canonicalCountLine(line string) (uint64, uint64, bool) {
	if line != strings.TrimSpace(line) {
		return 0, 0, false
	}
	fields := strings.Split(line, " ")
	if len(fields) != 4 || fields[0] != "Positive:" || fields[2] != "Negative:" {
		return 0, 0, false
	}
	positive, positiveOK := canonicalDecimal(fields[1])
	negative, negativeOK := canonicalDecimal(fields[3])
	return positive, negative, positiveOK && negativeOK
}

func canonicalStateCount(line string) (uint64, bool) {
	if line != strings.TrimSpace(line) || !strings.HasPrefix(line, "States ") {
		return 0, false
	}
	return canonicalDecimal(strings.TrimPrefix(line, "States "))
}
