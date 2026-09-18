// Negative results require an explicit forbidden row for every candidate.
// Counts alone or a concealed witness must not produce a proof result.
package isla

import "testing"

func TestNegativeResultRows(t *testing.T) {
	for _, row := range []string{"???;", "forbidden ???;"} {
		output := "Test q Forbidden\nStates 1\n" + row + "\nPositive: 0 Negative: 1"
		result, err := parseHerdResult(output, "")
		if err != nil || result.candidates != 1 || result.counterexampleState != "" {
			t.Errorf("negative result = %#v, %v", result, err)
		}
	}
}

func TestNegativeResultRejectsMissingOrWrongRows(t *testing.T) {
	for _, row := range []string{"", "allowed 0:x10=64;", "0:x10=64;", "forbidden 0:x10=64;"} {
		output := "Test q Forbidden\nStates 1\n" + row + "\nPositive: 0 Negative: 1"
		assertHerdResultError(t, output)
	}
	assertHerdResultError(t, "Test q Forbidden\nPositive: 0 Negative: 1")
}
