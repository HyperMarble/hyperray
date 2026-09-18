// Stored traces are mostly repeated solver text, so they are compressed.
package isla

import (
	"bytes"
	"compress/gzip"
)

// A trace is mostly repeated solver text, so it is stored compressed.
func pack(content []byte) ([]byte, error) {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	if _, err := writer.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func unpack(packed []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(reader); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
