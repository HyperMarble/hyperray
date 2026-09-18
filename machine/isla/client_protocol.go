// One isla-client process, connected once and asked for many instructions.
// It must report a closed or failed session, never an empty trace.
package isla

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Answer tags the isla-client protocol writes before each reply.
const (
	answerError       byte = 0
	answerVersion     byte = 1
	answerStartTraces byte = 2
	answerTrace       byte = 3
	answerEndTraces   byte = 4
)

// writeMessage sends one request as a little-endian length and its bytes.
func writeMessage(writer io.Writer, text string) error {
	body := []byte(text)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(body)))
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(body)
	return err
}

// readLengthPrefixed reads one little-endian length and that many bytes.
func readLengthPrefixed(reader io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(header)
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

// readAnswerTag reads the single byte naming the next reply.
func readAnswerTag(reader io.Reader) (byte, error) {
	tag := make([]byte, 1)
	if _, err := io.ReadFull(reader, tag); err != nil {
		return 0, err
	}
	return tag[0], nil
}

// ReadTraces collects every trace isla-client writes for one instruction.
//
// The reply is a start tag, any number of traces, then an end tag. An error
// tag ends the exchange with a failure, never with the traces read so far.
func ReadTraces(reader io.Reader) ([][]byte, error) {
	var traces [][]byte
	started := false
	for {
		tag, err := readAnswerTag(reader)
		if err != nil {
			return nil, err
		}
		switch tag {
		case answerStartTraces:
			started = true
		case answerTrace:
			if _, err := readAnswerTag(reader); err != nil {
				return nil, err
			}
			body, err := readLengthPrefixed(reader)
			if err != nil {
				return nil, err
			}
			traces = append(traces, body)
		case answerEndTraces:
			if !started {
				return nil, errors.New("isla-client ended traces it never started")
			}
			return traces, nil
		case answerError:
			return nil, errors.New("isla-client reported an error")
		default:
			return nil, fmt.Errorf("isla-client sent an unknown answer tag %d", tag)
		}
	}
}
