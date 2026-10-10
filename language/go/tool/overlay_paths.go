// Purpose: reads an overlay file the way go reads it: the file's own path,
// every disk path in it and every backing path are relative to the folder the
// build runs in (the project root), not to the overlay file's folder.
// Never:   reads a relative path from the folder the adapter was started in.
package tool

// ReadOverlayUnder is the Replace map of the overlay at path, with every
// disk path and every non-empty backing path made absolute under root.
func ReadOverlayUnder(root, path string) (map[string]string, error) {
	replace, err := ReadOverlay(under(root, path))
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(replace))
	for disk, backing := range replace {
		resolved[under(root, disk)] = backingUnder(root, backing)
	}
	return resolved, nil
}

// backingUnder leaves an empty backing path empty: that is a removal.
func backingUnder(root, backing string) string {
	if backing == "" {
		return ""
	}
	return under(root, backing)
}
