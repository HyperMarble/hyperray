// Position decoding tracks whether the frozen terminator flag was emitted.
// Omitted legacy flags infer terminators from nullable statement indexes.
package compilercatalog

import (
	"bytes"
	"encoding/json"
)

func (position *Position) UnmarshalJSON(content []byte) error {
	type positionWire Position
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var decoded positionWire
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return err
	}
	*position = Position(decoded)
	_, position.terminatorPresent = fields["terminator"]
	_, position.statementPresent = fields["statement"]
	return nil
}

func (position Position) terminatorMatches(expected bool) bool {
	if position.terminatorPresent {
		return position.Terminator == expected
	}
	// Manifest v1 records from the current Rust producer legitimately omit
	// this redundant flag. They must still retain the nullable statement field
	// so a malformed record with a missing statement cannot become a terminator.
	if !position.statementPresent {
		return false
	}
	if position.Terminator {
		return false
	}
	return true
}
