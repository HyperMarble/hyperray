// A replay must reproduce the exact reported input and output.
// Exit code one alone does not establish a requirement violation.
package nativecheck

import (
	"errors"
	"fmt"

	"github.com/HyperMarble/hyperray/execution"
)

func replayMatches(replay execution.Result, candidate Counterexample) (string, error) {
	if replay.Status != execution.Exited {
		return stopped(replay)
	}
	if replay.ExitCode != 1 {
		return "", fmt.Errorf("replay did not reproduce the violation: exit %d", replay.ExitCode)
	}
	count, err := observations(replay.Output)
	if err != nil {
		return "", err
	}
	if count != 1 {
		return "", errors.New("replay must contain exactly one observation")
	}
	actual, err := witness(replay.Output)
	if err != nil {
		return "", err
	}
	if actual != candidate {
		return "", errors.New("replay counterexample differs from search counterexample")
	}
	return "", nil
}
