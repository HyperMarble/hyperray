// Purpose: writes the overlay that puts one generated keep file into each
// package for the build, and a second one into the package's external
// test package when it has one.
// Never:   writes into the module's folder, or hides a file the package
// already has: the overlay only ever adds a name the package does not use.
package keep

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/module"
	"github.com/HyperMarble/hyperray/language/go/record"
	"github.com/HyperMarble/hyperray/language/go/tool"
)

// WriteOverlay writes one keep file per package into out and returns the
// path of the overlay that maps each into its package. Go takes one
// overlay, so a user's overlay, when there is one, is merged in first.
func WriteOverlay(packages []module.Package, out, userOverlay string) (string, error) {
	replace := map[string]string{}
	if userOverlay != "" {
		theirs, err := tool.ReadOverlay(userOverlay)
		if err != nil {
			return "", err
		}
		replace = theirs
	}
	user := map[string]string{}
	for disk, backing := range replace {
		user[disk] = backing
	}
	for index, pkg := range packages {
		if err := keepFilesFor(pkg, user, replace, out, index); err != nil {
			return "", err
		}
	}
	text, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		return "", record.Unreadable("overlay", err)
	}
	overlay := filepath.Join(out, "overlay.json")
	if err := os.WriteFile(overlay, text, 0o644); err != nil {
		return "", record.Unreadable(overlay, err)
	}
	return overlay, nil
}
