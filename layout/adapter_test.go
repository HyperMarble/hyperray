// This test permits only one adapter directory for each supported language.
// It must report each unknown directory instead of ignoring it.
package layout

import (
	"os"
	"sort"
	"strings"
	"testing"
)

var supportedAdapters = map[string]bool{
	"c": true, "cpp": true, "go": true, "python": true, "rust": true,
}

func TestAdapterDirectoryNames(t *testing.T) {
	failures, err := unsupportedAdapterDirectories("../adapters")
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Errorf("unsupported adapter directories:\n%s", strings.Join(failures, "\n"))
	}
}

func unsupportedAdapterDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var failures []string
	for _, entry := range entries {
		if !entry.IsDir() || supportedAdapters[entry.Name()] {
			continue
		}
		failures = append(failures, entry.Name())
	}
	sort.Strings(failures)
	return failures, nil
}
