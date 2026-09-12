// Result reporting states the engine's own status and its counterexample.
// It must never report a proof when the engine reported anything else.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/HyperMarble/hyperray/machine/isla"
	"github.com/spf13/cobra"
)

// writeMachineResult reports the status with its counterexample, so a reader
// learns which input broke the property rather than only that one exists.
func writeMachineResult(command *cobra.Command, result isla.ExecutableResult) error {
	report := map[string]any{
		"status":               string(result.Verification.Status),
		"query_name":           result.Verification.QueryName,
		"candidate_count":      result.Verification.CandidateCount,
		"counterexample_count": result.Verification.CounterexampleCount,
	}
	if result.Verification.CounterexampleState != "" {
		report["counterexample_state"] = result.Verification.CounterexampleState
	}
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write machine result: %w", err)
	}
	if result.Verification.Status != isla.Proved {
		return fmt.Errorf("status is %s", result.Verification.Status)
	}
	return nil
}

func decodeBackingBytes(text string) ([]byte, error) {
	if text == "" {
		return nil, nil
	}
	return hex.DecodeString(text)
}
