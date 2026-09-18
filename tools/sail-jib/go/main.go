// The command accepts a Sail JIB census only after structural validation.
// It never reports completed lowering for invalid input.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type report struct {
	Complete         bool `json:"complete"`
	Definitions      int  `json:"jib_definitions"`
	Origins          int  `json:"jib_origins"`
	InstructionKinds int  `json:"instruction_kinds"`
}

func operate(arguments []string, output io.Writer) error {
	if len(arguments) != 1 {
		return fmt.Errorf("usage: sail-jib-catalog FILE")
	}
	value, err := readCatalog(arguments[0])
	if err != nil {
		return err
	}
	if err := validateCatalog(value); err != nil {
		return err
	}
	result := report{
		Complete: true, Definitions: value.DefinitionCount,
		Origins:          len(value.Origins),
		InstructionKinds: len(value.InstructionKinds),
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return fmt.Errorf("write JIB catalog report: %w", err)
	}
	return nil
}

func main() {
	if err := operate(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
