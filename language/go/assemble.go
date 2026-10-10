// Purpose: the parts of the record that are not the build itself: what the
// machine and the go tool say, and what the build was asked for.
// Never:   reads instructions or proves anything.
package goadapter

import (
	"github.com/HyperMarble/hyperray/language/go/build"
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

// settingsOf is what was asked, as asked, what the build ran with, the pin
// files before and after, and every outside input, by hash.
func settingsOf(root string, asked, choice record.Choice, locks []record.FileDigest, facts *record.BuildRecord) (record.Settings, error) {
	flags, err := tool.BuildFlags(root, choice)
	if err != nil {
		return record.Settings{}, err
	}
	after, changed, err := module.PinsAfter(root, locks)
	if err != nil {
		return record.Settings{}, err
	}
	settings := record.Settings{
		Requested: asked, Tags: build.StampedTags(facts.Artifacts), BuildFlags: flags,
		LockFiles: locks, PinsChanged: changed,
	}
	if changed {
		settings.LockFilesAfter = after
	}
	return settings, tool.Inputs(root, flags, facts.Environment, &settings)
}
