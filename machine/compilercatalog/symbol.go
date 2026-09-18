// Symbol observation reads object and image tables as metadata only.
// A shared symbol name never proves an operation-to-instruction mapping.
package compilercatalog

import (
	"bytes"
	"debug/elf"
	"errors"
)

func symbols(content []byte) ([]string, error) {
	file, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	return symbolNames(file)
}

func symbolNames(file *elf.File) ([]string, error) {
	entries, err := file.Symbols()
	if errors.Is(err, elf.ErrNoSymbols) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "" {
			continue
		}
		if _, exists := seen[entry.Name]; exists {
			continue
		}
		seen[entry.Name] = struct{}{}
		result = append(result, entry.Name)
	}
	return result, nil
}
