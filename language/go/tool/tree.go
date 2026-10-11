// Purpose: hashes a folder of files the way the record needs it: every file
// in the folder by hash, or one hash over a whole tree, path and content
// both in, so a changed byte or a renamed file changes the answer.
// Never:   follows a link out of the tree, or skips a file it cannot read.
package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/language/go/record"
)

// digestFolder hashes every file directly inside folder, sorted by name.
func digestFolder(folder string) ([]record.FileDigest, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, record.Unreadable(folder, err)
	}
	found := []record.FileDigest{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		digest, err := record.DigestOf(filepath.Join(folder, entry.Name()))
		if err != nil {
			return nil, err
		}
		found = append(found, digest)
	}
	return found, nil
}

// treeHash is one SHA-256 over every file under root in sorted path order:
// for each file its path relative to root, a zero byte, its bytes, a zero
// byte. WalkDir visits in lexical order, which fixes the order.
func treeHash(root string) (string, error) {
	sum := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return hashFile(sum, relative, path)
	})
	if err != nil {
		return "", record.Unreadable(root, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func hashFile(sum io.Writer, relative, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.WriteString(sum, relative+"\x00"); err != nil {
		return err
	}
	if _, err := io.Copy(sum, file); err != nil {
		return err
	}
	_, err = io.WriteString(sum, "\x00")
	return err
}
