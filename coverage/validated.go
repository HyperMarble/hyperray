// ValidatedCoverage is an opaque snapshot minted only by a successful Check.
// Its zero value never exposes a coverage report.
package coverage

import "github.com/HyperMarble/hyperray/model"

// ValidatedCoverage carries a private immutable coverage snapshot.
type ValidatedCoverage struct {
	report    Report
	graph     model.Model
	roots     map[string]RootEntry
	validated bool
}

// ValidatedRoot identifies one selected root and its exact entry states.
type ValidatedRoot struct {
	ID       string   `json:"id"`
	StateIDs []string `json:"state_ids"`
}

// Report returns the complete report stored in a validated snapshot.
func (validated ValidatedCoverage) Report() (Report, error) {
	if !validated.validated {
		return Report{}, coverageError("invalid_validated_coverage", "zero_value")
	}
	return validated.report, nil
}

// Model returns a deep copy of the exact canonical model checked for coverage.
func (validated ValidatedCoverage) Model() (model.Model, error) {
	if !validated.validated {
		return model.Model{}, coverageError("invalid_validated_coverage", "zero_value")
	}
	return cloneModel(validated.graph), nil
}

// Roots returns canonical copies of the selected validated roots.
func (validated ValidatedCoverage) Roots(rootIDs []string) ([]ValidatedRoot, error) {
	if !validated.validated {
		return nil, coverageError("invalid_validated_coverage", "zero_value")
	}
	if len(rootIDs) == 0 {
		return nil, coverageError("empty_root_selection", "root_ids")
	}
	selected, err := uniqueIDSet(rootIDs, "root_selection")
	if err != nil {
		return nil, err
	}
	result := make([]ValidatedRoot, 0, len(selected))
	for _, rootID := range sortedIDs(selected) {
		entry, exists := validated.roots[rootID]
		if !exists {
			return nil, coverageError("unknown_root", rootID)
		}
		states := sortedIDs(stringIDSet(entry.StateIDs))
		result = append(result, ValidatedRoot{ID: rootID, StateIDs: states})
	}
	return result, nil
}
