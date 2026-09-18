// Expression parts retain quoted values and nested expressions as data.
// Delimiters inside literals never create instruction events.
package isla

import "strings"

func semanticTreeParts(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '(' || value[len(value)-1] != ')' {
		return nil, semanticProtocolError("expected tree expression")
	}
	body := value[1 : len(value)-1]
	parts := []string{}
	for index := 0; index < len(body); {
		if asciiSpace(body[index]) {
			index++
			continue
		}
		if body[index] == ';' {
			index = semanticTreeCommentEnd(body, index)
			continue
		}
		end, err := semanticTreePartEnd(body, index)
		if err != nil {
			return nil, err
		}
		parts = append(parts, body[index:end])
		index = end
	}
	if len(parts) == 0 {
		return nil, semanticProtocolError("empty tree expression")
	}
	return parts, nil
}

func semanticTreeCommentEnd(value string, start int) int {
	end := strings.IndexByte(value[start:], '\n')
	if end < 0 {
		return len(value)
	}
	return start + end + 1
}
