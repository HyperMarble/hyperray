//go:build execution_integration && darwin

// The research matrix names declared fixtures, never production dispatch rules.
package execution_test

import "path/filepath"

type researchCase struct {
	name       string
	executable string
	arguments  []string
	nonzero    bool
	markers    []string
}

func nativeCases(root string) []researchCase {
	var cases []researchCase
	for _, name := range []string{"stack_array", "mixed_width", "recursive_calls",
		"dynamic_call", "generic_calls", "combined"} {
		for _, optimization := range []string{"0", "2"} {
			label := name + "-o" + optimization
			cases = append(cases, researchCase{label, filepath.Join(root, label, "search"),
				[]string{"-m100", "-w18"}, false,
				[]string{"Full statespace search", "errors: 0", "COVERAGE inputs=65536 domain=65536"}})
		}
	}
	return append(cases,
		researchCase{"stored-pointer-bug", filepath.Join(root, "broken", "search"),
			[]string{"-m100", "-w18"}, false, []string{"COUNTEREXAMPLE input=65535", "errors: 1"}},
		researchCase{"stored-pointer-replay", filepath.Join(root, "broken", "replay"),
			[]string{"65535"}, true, []string{"COUNTEREXAMPLE input=65535"}})
}

func threadCases(root string) []researchCase {
	var cases []researchCase
	programs := [][2]string{{"lost_update", "both increments must remain"},
		{"publication", "ready must publish the payload"},
		{"heap_lock", "both updates must reach each heap cell"}}
	for _, program := range programs {
		executable := filepath.Join(root, "target", "release", program[0])
		cases = append(cases,
			researchCase{program[0] + "-fixed", executable, []string{"fixed"}, false,
				[]string{"SEARCH_COMPLETE executions="}},
			researchCase{program[0] + "-broken", executable, []string{"broken"}, true,
				[]string{program[1]}})
	}
	return cases
}
