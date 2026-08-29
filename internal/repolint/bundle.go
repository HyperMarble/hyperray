// Bundle completeness: the canonical runner (./test.sh) ships inside the
// test patch; a regenerated patch that drops it passes locally and fails
// every container verify. Checked from the patch text alone.
package repolint

import (
	"regexp"
	"strings"
)

var patchFilePattern = regexp.MustCompile(`(?m)^diff --git a/(\S+) b/(\S+)$`)

// PatchFiles lists the file paths a unified git diff touches.
func PatchFiles(patch string) []string {
	seen := map[string]bool{}
	var files []string
	for _, match := range patchFilePattern.FindAllStringSubmatch(patch, -1) {
		path := match[2]
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}

// MissingBundleFiles returns the required files the patch does not touch.
func MissingBundleFiles(patch string, required []string) []string {
	touched := map[string]bool{}
	for _, file := range PatchFiles(patch) {
		touched[file] = true
	}
	var missing []string
	for _, name := range required {
		name = strings.TrimSpace(name)
		if name != "" && !touched[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
