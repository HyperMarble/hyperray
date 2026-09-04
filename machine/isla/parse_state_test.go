// State tests accept one concrete counterexample and reject ambiguous output.
// They must not guess which state satisfied a mixed result.
package isla

import "testing"

func TestResultState(t *testing.T) {
	state, err := resultState([]string{"States 1", "x=1;"}, 1, 0)
	if err != nil {
		t.Fatalf("resultState() error = %v", err)
	}
	if state != "x=1;" {
		t.Errorf("resultState() = %q", state)
	}
	empty, err := resultState(nil, 0, 1)
	if err != nil || empty != "" {
		t.Errorf("resultState(no counterexample) = %q, %v", empty, err)
	}
}

func TestResultStateErrors(t *testing.T) {
	cases := []struct {
		lines    []string
		positive uint64
		negative uint64
		code     ErrorCode
	}{
		{lines: nil, positive: 1, negative: 1, code: ResultError},
		{lines: nil, positive: 1, code: ProtocolError},
		{lines: []string{"States 1", "???;"}, positive: 1, code: ProtocolError},
		{lines: []string{"States 1", ""}, positive: 1, code: ProtocolError},
	}
	for index := range cases {
		testCase := cases[index]
		state, err := resultState(testCase.lines, testCase.positive, testCase.negative)
		if err == nil {
			t.Errorf("resultState() = %q, nil error", state)
		}
		assertInternalCode(t, err, testCase.code)
	}
}
