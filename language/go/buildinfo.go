// Purpose: reads Go's own record of a build back out of the built file: the
// Go version, every module version with its hash, every setting.
// Never:   fills in a fact Go did not write into the file.
package goadapter

import (
	"debug/buildinfo"
	"runtime/debug"
)

func readBuildInfo(path string) (BuildInfo, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return BuildInfo{}, unreadable(path, err)
	}
	found := BuildInfo{
		GoVersion: info.GoVersion,
		Path:      info.Path,
		Main:      moduleOf(&info.Main),
		Deps:      []Module{},
		Settings:  []Setting{},
	}
	for _, dep := range info.Deps {
		found.Deps = append(found.Deps, moduleOf(dep))
	}
	for _, setting := range info.Settings {
		found.Settings = append(found.Settings, Setting{Key: setting.Key, Value: setting.Value})
	}
	return found, nil
}

func moduleOf(module *debug.Module) Module {
	found := Module{Path: module.Path, Version: module.Version, Sum: module.Sum}
	if module.Replace != nil {
		replacement := moduleOf(module.Replace)
		found.Replace = &replacement
	}
	return found
}
