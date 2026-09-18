// Reading one strict JSON file for a command. A file with unknown fields or
// trailing data is rejected, never read in part.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func decodeJSONFile(path string, value any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("JSON input contains trailing data")
	}
	return nil
}
