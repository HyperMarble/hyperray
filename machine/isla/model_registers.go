// The registers a model declares, read from the model itself. A copied list
// goes stale the day the model changes; the model cannot.
package isla

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// ModelRegisterNames returns every register the Sail model at path declares,
// in source spelling.
//
// A Sail model compiled by isla-sail declares each register on one line as
// `register <encoded-name> : <type>`. The name is z-encoded by Sail's
// `Util.zencode_string`; DecodeSailName reverses that.
func ModelRegisterNames(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, engineError(InvalidInput, path, err.Error())
	}
	names := make([]string, 0, 1024)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for scanner.Scan() {
		encoded, declares := registerDeclaration(scanner.Text())
		if !declares {
			continue
		}
		name, err := DecodeSailName(encoded)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, engineError(InvalidInput, path, err.Error())
	}
	if len(names) == 0 {
		return nil, engineError(InvalidInput, path, "model declares no register")
	}
	return names, nil
}

// registerDeclaration returns the encoded name from a `register` line.
func registerDeclaration(line string) (string, bool) {
	rest, found := strings.CutPrefix(line, "register ")
	if !found {
		return "", false
	}
	name, _, typed := strings.Cut(rest, " :")
	if !typed || name == "" {
		return "", false
	}
	return name, true
}

// DecodeSailName reverses Sail's z-encoding of an identifier.
//
// The encoding, from Sail's `Util.zchar`: a leading `z` marks an encoded
// name; `zz` is a literal `z`; `_`, digits, and letters other than `z` pass
// through; every other character becomes `z` followed by one shifted
// character.
func DecodeSailName(encoded string) (string, error) {
	rest, encodedName := strings.CutPrefix(encoded, "z")
	if !encodedName {
		return "", engineError(ProtocolError, "sail name", "not z-encoded: "+encoded)
	}
	var decoded strings.Builder
	for index := 0; index < len(rest); index++ {
		character := rest[index]
		if character != 'z' {
			decoded.WriteByte(character)
			continue
		}
		index++
		if index >= len(rest) {
			return "", engineError(ProtocolError, "sail name", "truncated escape: "+encoded)
		}
		original, valid := decodeSailEscape(rest[index])
		if !valid {
			return "", engineError(ProtocolError, "sail name", "unknown escape: "+encoded)
		}
		decoded.WriteByte(original)
	}
	return decoded.String(), nil
}

// decodeSailEscape maps the character after an escaping `z` back to the
// original. It is the inverse of the shifts in `Util.zchar`, checked by
// enumerating every printable character through that function.
func decodeSailEscape(shifted byte) (byte, bool) {
	switch {
	case shifted == 'z':
		return 'z', true
	case shifted >= '0' && shifted <= '9':
		return shifted - 16, true
	case shifted >= 'A' && shifted <= 'F':
		return shifted - 23, true
	case shifted >= 'G' && shifted <= 'M':
		return shifted - 13, true
	case shifted >= 'N' && shifted <= 'Q':
		return shifted + 13, true
	case shifted == 'S':
		return '`', true
	case shifted >= 'T' && shifted <= 'W':
		return shifted + 39, true
	default:
		return 0, false
	}
}
