// A stored trace written as its distinct lines and the order they appeared.
// A trace repeats most of its own lines, so naming each one once is smaller
// than the text and readable without unpacking.
package isla

import (
	"fmt"
	"strconv"
	"strings"
)

// dictionaryEncode returns the distinct lines of text and the order they
// appeared, separated by a blank line.
//
// Reading back gives the identical text.
func dictionaryEncode(text string) string {
	lines := strings.Split(text, "\n")
	position := make(map[string]int, len(lines))
	distinct := make([]string, 0, len(lines))
	order := make([]string, 0, len(lines))
	for _, line := range lines {
		index, seen := position[line]
		if !seen {
			index = len(distinct)
			position[line] = index
			distinct = append(distinct, line)
		}
		order = append(order, strconv.Itoa(index))
	}
	return strings.Join(distinct, "\n") + "\n\x00\n" + strings.Join(order, ",")
}

// dictionaryDecode restores the text a dictionary was built from.
//
// A malformed dictionary is reported, never decoded into partial text.
func dictionaryDecode(encoded string) (string, error) {
	parts := strings.SplitN(encoded, "\n\x00\n", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("dictionary has no order section")
	}
	distinct := strings.Split(parts[0], "\n")
	lines := make([]string, 0, len(distinct))
	for _, field := range strings.Split(parts[1], ",") {
		index, err := strconv.Atoi(field)
		if err != nil {
			return "", fmt.Errorf("dictionary order %q is not a number", field)
		}
		if index < 0 || index >= len(distinct) {
			return "", fmt.Errorf("dictionary order %d names no line", index)
		}
		lines = append(lines, distinct[index])
	}
	return strings.Join(lines, "\n"), nil
}
