package testir

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/HyperMarble/hyperray/internal/executor"
)

// SemanticIsolationEvidence proves that materialization/retranslation ran in
// a fresh byte-identical workspace and that neither the isolated copy nor a
// translator side effect survived the vector.
type SemanticIsolationEvidence struct {
	SourceRoot               string `json:"source_root"`
	SourceSHA256             string `json:"source_sha256"`
	IsolatedRoot             string `json:"isolated_root"`
	InitialIsolatedSHA256    string `json:"initial_isolated_sha256"`
	RestoredIsolatedSHA256   string `json:"restored_isolated_sha256"`
	OriginalAfterSHA256      string `json:"original_after_sha256"`
	IsolatedWorkspaceRemoved bool   `json:"isolated_workspace_removed"`
}

type semanticWorkspace struct {
	parent   string
	root     string
	evidence SemanticIsolationEvidence
}

func makeSemanticWorkspace(sourceRoot, expectedDigest string) (*semanticWorkspace, error) {
	current, err := executor.WorkspaceDigest(sourceRoot)
	if err != nil {
		return nil, err
	}
	if current != expectedDigest {
		return nil, fmt.Errorf("frozen workspace became stale: got %s, want %s", current, expectedDigest)
	}
	parent, err := os.MkdirTemp("", "hyperray-testir-semantic-*")
	if err != nil {
		return nil, err
	}
	root := filepath.Join(parent, "workspace")
	if err := copySemanticTree(sourceRoot, root); err != nil {
		_ = os.RemoveAll(parent)
		return nil, err
	}
	copied, err := executor.WorkspaceDigest(root)
	if err != nil || copied != expectedDigest {
		_ = os.RemoveAll(parent)
		if err == nil {
			err = fmt.Errorf("isolated semantic workspace digest is %s, want %s", copied, expectedDigest)
		}
		return nil, err
	}
	return &semanticWorkspace{parent: parent, root: root, evidence: SemanticIsolationEvidence{
		SourceRoot: sourceRoot, SourceSHA256: expectedDigest, IsolatedRoot: root, InitialIsolatedSHA256: copied,
	}}, nil
}

func (workspace *semanticWorkspace) close() error {
	if workspace == nil {
		return fmt.Errorf("semantic workspace is nil")
	}
	isolatedDigest, isolatedErr := executor.WorkspaceDigest(workspace.root)
	workspace.evidence.RestoredIsolatedSHA256 = isolatedDigest
	originalDigest, originalErr := executor.WorkspaceDigest(workspace.evidence.SourceRoot)
	workspace.evidence.OriginalAfterSHA256 = originalDigest
	removeErr := os.RemoveAll(workspace.parent)
	_, statErr := os.Lstat(workspace.parent)
	workspace.evidence.IsolatedWorkspaceRemoved = removeErr == nil && os.IsNotExist(statErr)
	if isolatedErr != nil {
		return fmt.Errorf("digest restored isolated semantic workspace: %w", isolatedErr)
	}
	if isolatedDigest != workspace.evidence.InitialIsolatedSHA256 {
		return fmt.Errorf("semantic translation left workspace side effects: got %s, want %s", isolatedDigest, workspace.evidence.InitialIsolatedSHA256)
	}
	if originalErr != nil || originalDigest != workspace.evidence.SourceSHA256 {
		return fmt.Errorf("original frozen workspace changed during semantic isolation")
	}
	if removeErr != nil || !workspace.evidence.IsolatedWorkspaceRemoved {
		return fmt.Errorf("remove isolated semantic workspace: %v", removeErr)
	}
	return nil
}

func copySemanticTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace entry %q has unsupported mode %s", relative, info.Mode())
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err = file.Write(body); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	})
}
