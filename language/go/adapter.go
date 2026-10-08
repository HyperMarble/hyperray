// Purpose: the Go adapter: build a module its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
package goadapter

import (
	"github.com/HyperMarble/hyperray/language/go/build"
	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// BuildRecordOf builds the module at root the way choice asks, puts every
// output under out, and returns the build record or the reason there is none.
func BuildRecordOf(root, out string, choice record.Choice) record.Outcome {
	facts, err := tryBuildRecord(root, out, choice)
	if err != nil {
		return record.Outcome{Status: "blocked", Reason: err.Error()}
	}
	return record.Outcome{Status: "built", BuildRecord: facts}
}

func tryBuildRecord(root, out string, choice record.Choice) (*record.BuildRecord, error) {
	asked := choice
	root, choice = tool.Relocated(root, choice)
	locks, err := module.LockFiles(root)
	if err != nil {
		return nil, err
	}
	packages, err := module.ListPackages(root, choice)
	if err != nil {
		return nil, err
	}
	files, notBuilt, err := build.BuildAll(root, out, choice, packages)
	if err != nil {
		return nil, err
	}
	artifacts, err := build.ArtifactsOf(files)
	if err != nil {
		return nil, err
	}
	facts, err := machineFacts(root)
	if err != nil {
		return nil, err
	}
	facts.Artifacts = artifacts
	facts.NotBuilt = notBuilt
	facts.NativeCode = build.NativeCodeOf(packages)
	facts.Settings, err = settingsOf(root, asked, choice, locks, facts)
	if err != nil {
		return nil, err
	}
	facts.LocalModules, err = module.LocalModules(root, choice)
	if err != nil {
		return nil, err
	}
	facts.Excluded, err = module.ExcludedFolders(root, choice, packages)
	if err != nil {
		return nil, err
	}
	return withNested(root, out, choice, facts)
}
