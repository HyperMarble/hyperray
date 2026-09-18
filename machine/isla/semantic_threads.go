// Thread parsing validates every numbered event tree before it records events.
// Thread numbers must form one exact zero-based sequence.
package isla

import (
	"strings"
)

type semanticThreads struct {
	count        uint64
	traceCount   uint64
	eventCount   uint64
	encodings    map[string]struct{}
	instructions map[SemanticInstruction]struct{}
	records      []SemanticThread
}

func parseSemanticThreads(lines []string) (semanticThreads, error) {
	result := semanticThreads{
		encodings: make(map[string]struct{}), instructions: make(map[SemanticInstruction]struct{}),
	}
	for index := 0; index < len(lines); {
		if strings.TrimSpace(lines[index]) == "" {
			index++
			continue
		}
		thread, err := semanticThreadNumber(lines[index])
		if err != nil || thread != result.count {
			return semanticThreads{}, semanticProtocolError("invalid thread sequence")
		}
		index++
		start := index
		for index < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[index]), "Thread ") {
			index++
		}
		if err := result.addTree(strings.Join(lines[start:index], "\n")); err != nil {
			return semanticThreads{}, err
		}
		result.count++
	}
	if result.count == 0 {
		return semanticThreads{}, semanticProtocolError("missing thread tree")
	}
	return result, nil
}
