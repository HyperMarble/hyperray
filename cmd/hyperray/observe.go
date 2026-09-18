// Observe exposes diagnostic execution without authorizing a proof verdict.
// It must report resource and worker errors after preserving the JSON result.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/HyperMarble/hyperray/execution"
	"github.com/spf13/cobra"
)

func newObserveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "observe REQUEST.json",
		Short: "Observe a trusted checker; this command does not prove correctness",
		Args:  cobra.ExactArgs(1),
		RunE:  executeObservation,
	}
}

func executeObservation(command *cobra.Command, arguments []string) error {
	var request execution.Request
	if err := decodeJSONFile(arguments[0], &request); err != nil {
		return fmt.Errorf("read observation request: %w", err)
	}
	result, err := execution.Run(command.Context(), request)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(command.OutOrStdout()).Encode(result); err != nil {
		return fmt.Errorf("write observation result: %w", err)
	}
	if result.Status != execution.Exited || result.ExitCode != 0 {
		return fmt.Errorf("checker status %s, exit code %d", result.Status, result.ExitCode)
	}
	return nil
}
