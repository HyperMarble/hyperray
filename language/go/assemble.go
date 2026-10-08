// Purpose: the parts of the record that are not the build itself: what the
// machine and the go tool say, and the nested modules built after the root.
// Never:   reads instructions or proves anything.
package goadapter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// machineFacts is the part of the record that comes from the machine and
// the go tool, not from the build itself.
func machineFacts(root string) (*record.BuildRecord, error) {
	goTool, err := tool.GoToolchain(root)
	if err != nil {
		return nil, err
	}
	environment, err := tool.GoEnvironment(root)
	if err != nil {
		return nil, err
	}
	cTool, err := tool.CCompiler(root, environment)
	if err != nil {
		return nil, err
	}
	build, err := tool.OsBuild(root)
	if err != nil {
		return nil, err
	}
	return &record.BuildRecord{Language: "go", Toolchain: goTool, Environment: environment, CToolchain: cTool, OsBuild: build}, nil
}

// withNested builds every nested module into its own folder under out and
// hashes the go.mod of every ignored one.
func withNested(root, out string, choice record.Choice, facts *record.BuildRecord) (*record.BuildRecord, error) {
	toBuild, ignored, err := module.NestedModules(root)
	if err != nil {
		return nil, err
	}
	for index, dir := range toBuild {
		nestedOut := filepath.Join(out, fmt.Sprintf("nested_%d", index))
		if err := os.MkdirAll(nestedOut, 0o755); err != nil {
			return nil, record.Unreadable(nestedOut, err)
		}
		nested, err := tryBuildRecord(dir, nestedOut, choice)
		if err != nil {
			return nil, err
		}
		facts.Nested = append(facts.Nested, record.Nested{Dir: dir, Record: nested})
	}
	for _, dir := range ignored {
		digest, err := record.DigestOf(filepath.Join(dir, "go.mod"))
		if err != nil {
			return nil, err
		}
		facts.IgnoredModules = append(facts.IgnoredModules, digest)
	}
	return facts, nil
}
