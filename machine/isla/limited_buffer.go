// Limited buffers bound memory used for external-tool output.
// They consume excess bytes and expose the limit breach to the caller.
package isla

type limitedBuffer struct {
	content  []byte
	limit    uint64
	exceeded bool
}

func newLimitedBuffer(limit uint64) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (buffer *limitedBuffer) Write(content []byte) (int, error) {
	consumed := len(content)
	remaining := buffer.remaining()
	if remaining == 0 {
		buffer.exceeded = true
		return consumed, nil
	}
	accepted := uint64(consumed)
	if accepted > remaining {
		accepted = remaining
		buffer.exceeded = true
	}
	buffer.content = append(buffer.content, content[:accepted]...)
	return consumed, nil
}

func (buffer *limitedBuffer) remaining() uint64 {
	used := uint64(len(buffer.content))
	if used >= buffer.limit {
		return 0
	}
	return buffer.limit - used
}

func (buffer *limitedBuffer) String() string {
	return string(buffer.content)
}
