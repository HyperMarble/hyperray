// Purpose: walks a module for every folder holding Go code, outside nested
// modules and outside the folders Go's own rules skip.
// Never:   enters vendor, testdata, a dotted or underscored folder, or
// another module.
package module

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// goFolders is every folder under root holding a .go file, outside nested
// modules and outside the folders Go's own rules skip, in walk order.
func goFolders(root string, nested []string) []string {
	seen, dirs := map[string]bool{}, []string{}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() && path != root && (ignoredPath(root, path) || inAny(path, nested)) {
			return fs.SkipDir
		}
		dir := filepath.Dir(path)
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
		return nil
	})
	return dirs
}

func inAny(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
