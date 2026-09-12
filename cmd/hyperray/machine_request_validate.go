// Request validation names every missing field before any tool runs.
// It must never infer a boundary the caller did not declare.
package main

import (
	"errors"
	"fmt"
)

func (request machineRequest) validate() error {
	for name, value := range map[string]string{
		"binary":             request.Binary,
		"tools.solver":       request.Tools.Solver,
		"tools.semantics":    request.Tools.Semantics,
		"tools.footprints":   request.Tools.Footprints,
		"tools.architecture": request.Tools.Architecture,
		"tools.manifest":     request.Tools.Manifest,
		"negated_assertion":  request.Assertion,
	} {
		if value == "" {
			return fmt.Errorf("%s is empty", name)
		}
	}
	if request.Boundary.FunctionEnd <= request.Boundary.FunctionStart {
		return errors.New("boundary.function_end must exceed boundary.function_start")
	}
	if len(request.Memory.Mappings) == 0 {
		return errors.New("memory.mappings is empty, so no loaded byte is covered")
	}
	return nil
}
