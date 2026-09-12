// Request reading rejects an unknown field instead of ignoring it.
// A relative binary path resolves from the request file, not the shell.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func readMachineRequest(path string) (machineRequest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return machineRequest{}, fmt.Errorf("read machine request: %w", err)
	}
	var request machineRequest
	decoder := json.NewDecoder(bytesReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return machineRequest{}, fmt.Errorf("decode machine request: %w", err)
	}
	if err := request.validate(); err != nil {
		return machineRequest{}, fmt.Errorf("machine request: %w", err)
	}
	if !filepath.IsAbs(request.Binary) {
		request.Binary = filepath.Join(filepath.Dir(path), request.Binary)
	}
	return request, nil
}
