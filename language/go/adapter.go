// Purpose: the Go adapter: build a module its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
package goadapter

import "slices"

// BuildRecordOf builds the module at root the way choice asks, puts every
// output under out, and returns the build record or the reason there is none.
func BuildRecordOf(root, out string, choice Choice) Outcome {
	record, err := tryBuildRecord(root, out, choice)
	if err != nil {
		return Outcome{Status: "blocked", Reason: err.Error()}
	}
	return Outcome{Status: "built", BuildRecord: record}
}

func tryBuildRecord(root, out string, choice Choice) (*BuildRecord, error) {
	locks, err := lockFiles(root)
	if err != nil {
		return nil, err
	}
	if err := validateChoice(root, choice); err != nil {
		return nil, err
	}
	packages, err := listPackages(root, choice)
	if err != nil {
		return nil, err
	}
	files, err := buildAll(root, out, choice, packages)
	if err != nil {
		return nil, err
	}
	artifacts, err := artifactsOf(files)
	if err != nil {
		return nil, err
	}
	record, err := machineFacts(root)
	if err != nil {
		return nil, err
	}
	record.Artifacts = artifacts
	record.Settings = Settings{Requested: choice, LockFiles: locks}
	record.NativeCode = nativeCodeOf(packages)
	latest, err := lockFiles(root)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(locks, latest) {
		return nil, Blocked{Reason: "module pin files changed during build"}
	}
	return withNested(root, out, choice, record)
}

// machineFacts is the part of the record that comes from the machine and
// the go tool, not from the build itself.
func machineFacts(root string) (*BuildRecord, error) {
	tool, err := toolchain(root)
	if err != nil {
		return nil, err
	}
	environment, err := goEnvironment(root)
	if err != nil {
		return nil, err
	}
	cTool, err := cToolchain(root, environment)
	if err != nil {
		return nil, err
	}
	build, err := osBuild(root)
	if err != nil {
		return nil, err
	}
	return &BuildRecord{Language: "go", Toolchain: tool, Environment: environment, CToolchain: cTool, OsBuild: build}, nil
}
