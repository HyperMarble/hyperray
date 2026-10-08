// Purpose: reads a -overlay file and hashes it with every file it maps into
// the build, in sorted order, so the record names each byte an overlay
// brought in.
// Never:   hashes a file an overlay hides; an empty backing path is a removal.
package tool

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// ReadOverlay is the Replace map of an overlay file: disk path to backing
// path, an empty backing path meaning the disk file is hidden.
func ReadOverlay(path string) (map[string]string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, record.Unreadable(path, err)
	}
	var overlay struct{ Replace map[string]string }
	if err := json.Unmarshal(text, &overlay); err != nil {
		return nil, record.Unreadable(path, err)
	}
	return overlay.Replace, nil
}

// overlayOf hashes the overlay file and every file it maps into the build.
func overlayOf(root, path string) (*record.Overlay, error) {
	path = under(root, path)
	file, err := record.DigestOf(path)
	if err != nil {
		return nil, err
	}
	replace, err := ReadOverlay(path)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(replace))
	for key := range replace {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	backing := []record.FileDigest{}
	for _, key := range keys {
		digest, err := backingOf(root, replace[key])
		if err != nil {
			return nil, err
		}
		if digest != nil {
			backing = append(backing, *digest)
		}
	}
	return &record.Overlay{File: file, Backing: backing}, nil
}

func backingOf(root, target string) (*record.FileDigest, error) {
	if target == "" {
		return nil, nil
	}
	digest, err := record.DigestOf(under(root, target))
	return &digest, err
}
