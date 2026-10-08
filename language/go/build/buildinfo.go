// Purpose: reads Go's own record of a build back out of the built file: the
// Go version, every module version with its hash, every setting.
// Never:   fills in a fact Go did not write into the file.
package build

import (
	"debug/buildinfo"
	"runtime/debug"

	"github.com/HyperMarble/hyperray/language/go/record"
)

func ReadBuildInfo(path string) (record.BuildInfo, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return record.BuildInfo{}, record.Unreadable(path, err)
	}
	found := record.BuildInfo{
		GoVersion: info.GoVersion,
		Path:      info.Path,
		Main:      moduleOf(&info.Main),
		Deps:      []record.Module{},
		Settings:  []record.Setting{},
	}
	for _, dep := range info.Deps {
		found.Deps = append(found.Deps, moduleOf(dep))
	}
	for _, setting := range info.Settings {
		found.Settings = append(found.Settings, record.Setting{Key: setting.Key, Value: setting.Value})
	}
	return found, nil
}

func moduleOf(module *debug.Module) record.Module {
	found := record.Module{Path: module.Path, Version: module.Version, Sum: module.Sum}
	if module.Replace != nil {
		replacement := moduleOf(module.Replace)
		found.Replace = &replacement
	}
	return found
}
