// Bounded output retains diagnostics and stops a worker that exceeds its limit.
// Truncation must never masquerade as a complete log.
package execution

import (
	"bytes"
	"context"
)

type boundedOutput struct {
	content  bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (output *boundedOutput) Write(content []byte) (int, error) {
	retained := retainedBytes(output.limit-output.content.Len(), len(content))
	if _, err := output.content.Write(content[:retained]); err != nil {
		return 0, err
	}
	if retained != len(content) {
		output.exceeded = true
		output.cancel()
	}
	return len(content), nil
}

func retainedBytes(remaining, incoming int) int {
	if incoming > remaining {
		return remaining
	}
	return incoming
}
