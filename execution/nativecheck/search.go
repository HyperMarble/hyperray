// Assess a trusted native search report without granting a proof capability.
// Observation totals are consistency checks, not independent coverage evidence.
package nativecheck

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/execution"
)

func assess(request Request, search execution.Result) (Result, error) {
	result := Result{Status: Incomplete, Search: search}
	reason, err := stopped(search)
	if err != nil || reason != "" {
		result.Reason = reason
		return result, err
	}
	failures, err := searchErrors(search.Output)
	if err != nil {
		return result, err
	}
	if failures != 0 {
		candidate, err := witness(search.Output)
		result.Counterexample = &candidate
		return result, err
	}
	if strings.Contains(search.Output, "COUNTEREXAMPLE") {
		return result, errors.New("counterexample contradicts the zero-error summary")
	}
	if reason := partialSearch(search.Output); reason != "" {
		result.Reason = reason
		return result, nil
	}
	if err := completion(request, search.Output); err != nil {
		return result, err
	}
	result.Status = SearchReportedComplete
	return result, nil
}

func searchErrors(output string) (uint64, error) {
	fields, err := record(output, "State-vector", `^State-vector [0-9]+ byte, depth reached [0-9]+, errors: ([0-9]+)$`)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(fields[0], 10, 64)
}

func completion(request Request, output string) error {
	for _, line := range []string{
		"Full statespace search for:", "assertion violations +",
		"never claim - (none specified)", "invalid end states +",
		"cycle checks - (disabled by -DSAFETY)",
	} {
		if err := requiredLine(output, line); err != nil {
			return err
		}
	}
	count, err := observations(output)
	if err != nil {
		return err
	}
	span := request.Maximum - request.Minimum
	if span == math.MaxUint64 || count != span+1 {
		return fmt.Errorf("observation count %d does not cover the declared interval", count)
	}
	return nil
}
