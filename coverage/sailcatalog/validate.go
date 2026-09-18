// Validate requires exact declaration, decoder, and execution set equality.
// One missing or extra family is an explicit error.
package sailcatalog

import (
	"bytes"
	"encoding/json"
	"io"
)

func Validate(data []byte) (Report, error) {
	var value Catalog
	if err := decodeCatalog(data, &value); err != nil {
		return Report{}, err
	}
	declared, err := catalogSet(value.Declarations, "instruction_declarations")
	if err != nil {
		return Report{}, err
	}
	if len(declared) == 0 {
		return Report{}, catalogError("empty_instruction_catalog")
	}
	decoded, err := catalogSet(value.Decoded, "decode_clauses")
	if err != nil {
		return Report{}, err
	}
	executed, err := catalogSet(value.Executed, "execute_clauses")
	if err != nil {
		return Report{}, err
	}
	if err := requireExact(declared, decoded, "decode"); err != nil {
		return Report{}, err
	}
	if err := requireExact(declared, executed, "execute"); err != nil {
		return Report{}, err
	}
	return Report{Complete: true, Families: len(declared)}, nil
}

func decodeCatalog(data []byte, value *Catalog) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return catalogError("invalid_json", err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return catalogError("invalid_json", "trailing data")
	}
	return nil
}

func requireExact(required map[string]struct{}, actual map[string]struct{}, kind string) error {
	if values := missing(required, actual); len(values) != 0 {
		return catalogError("missing_"+kind+"_clause", values...)
	}
	if values := missing(actual, required); len(values) != 0 {
		return catalogError("extra_"+kind+"_clause", values...)
	}
	return nil
}
