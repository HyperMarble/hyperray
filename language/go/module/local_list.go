// Purpose: lists production dependencies for the whole module and test-only
// imports for test programs that successfully linked.
// Never:   asks Go to load tests whose build failed.
package module

import (
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

func successfulTestPackages(artifacts []record.Artifact) []string {
	tests := []string{}
	for _, artifact := range artifacts {
		if artifact.Kind == "test program" {
			tests = append(tests, artifact.Package)
		}
	}
	return tests
}

func localDependencies(root string, choice record.Choice, paths []string, tests bool) ([]Package, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	args := []string{"list", "-deps"}
	if tests {
		args = append(args, "-test")
	}
	args = append(args, listedFields)
	args = append(args, choice.Args()...)
	text, err := tool.Printed(root, "go", append(args, paths...)...)
	if err != nil {
		return nil, err
	}
	return DecodePackages(text)
}
