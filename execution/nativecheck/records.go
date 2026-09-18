// Parse the exact records emitted by SPIN and the generated observer.
// Missing, duplicate, malformed, or overflowing fields must remain errors.
package nativecheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func record(output, prefix, grammar string) ([]string, error) {
	var records []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			records = append(records, line)
		}
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("expected one %q record, got %d", prefix, len(records))
	}
	pattern, err := regexp.Compile(grammar)
	if err != nil {
		return nil, fmt.Errorf("record grammar: %w", err)
	}
	fields := pattern.FindStringSubmatch(records[0])
	if fields == nil {
		return nil, fmt.Errorf("malformed %q record", prefix)
	}
	return fields[1:], nil
}

func observations(output string) (uint64, error) {
	fields, err := record(output, "OBSERVATIONS", `^OBSERVATIONS count=([0-9]+) overflow=0$`)
	if err != nil {
		return 0, err
	}
	count, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("observation count: %w", err)
	}
	return count, nil
}

func witness(output string) (Counterexample, error) {
	fields, err := record(output, "COUNTEREXAMPLE", `^COUNTEREXAMPLE input=([0-9]+) output=([0-9]+)$`)
	if err != nil {
		return Counterexample{}, err
	}
	input, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return Counterexample{}, fmt.Errorf("counterexample input: %w", err)
	}
	value, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return Counterexample{}, fmt.Errorf("counterexample output: %w", err)
	}
	return Counterexample{Input: input, Output: value}, nil
}
