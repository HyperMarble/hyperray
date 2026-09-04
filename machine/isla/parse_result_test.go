// Full-parser tests make each nested parse error leave through one typed result.
// They must not return partial result data.
package isla

import "testing"

func TestParseHerdResultErrors(t *testing.T) {
	cases := []struct {
		name   string
		output string
	}{
		{name: "header", output: "bad"},
		{name: "counts", output: "Test q Allowed\nStates 1\nx=1;"},
		{name: "state", output: "Test q Allowed\nStates 1\n???;\nPositive: 1 Negative: 0"},
		{name: "relation", output: "Test q Allowed\nStates 1\n???;\nPositive: 0 Negative: 1"},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			result, err := parseHerdResult(testCase.output, "")
			if err == nil {
				t.Errorf("parseHerdResult() = %#v, nil error", result)
			}
		})
	}
}
