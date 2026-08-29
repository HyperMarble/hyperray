package rust

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func validateRustWorkspace(request semanticir.FrontendRequest) []semanticir.Diagnostic {
	workspace := request.Workspace
	whole := wholeSpan(request.Source)
	var diagnostics []semanticir.Diagnostic
	add := func(code semanticir.DiagnosticCode, message string) {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, code, message))
	}
	if workspace.ID == "" || workspace.Root == "" || workspace.WorkingDirectory == "" || workspace.BuildCommand == "" {
		add(semanticir.DiagnosticInvalidInput, "frozen Rust workspace ID, root, working directory, and build command are required")
		return diagnostics
	}
	if !filepath.IsAbs(workspace.Root) {
		add(semanticir.DiagnosticInvalidInput, "Rust workspace root must be absolute")
		return diagnostics
	}
	if !semanticir.ValidDigest(workspace.TreeDigest) {
		add(semanticir.DiagnosticInvalidInput, "Rust workspace tree digest is missing or malformed")
	}
	environmentDigest, environmentErr := semanticir.Digest(workspace.Environment)
	if environmentErr != nil || workspace.EnvironmentDigest != environmentDigest || !workspace.ClearEnvironment || !workspace.KillProcessGroup {
		add(semanticir.DiagnosticInvalidInput, "Rust workspace lacks an exact clear-environment/process-group compiler execution contract")
	}
	switch workspace.State {
	case semanticir.WorkspaceBaseOldTests, semanticir.WorkspaceBaseNewTests, semanticir.WorkspaceSolutionNewTests:
	default:
		add(semanticir.DiagnosticInvalidInput, fmt.Sprintf("Rust workspace state %q is invalid", workspace.State))
	}
	workingDirectory, ok := withinRustWorkspace(workspace.Root, workspace.WorkingDirectory)
	if !ok {
		add(semanticir.DiagnosticInvalidInput, "Rust workspace working directory escapes its root")
	} else if info, err := os.Stat(workingDirectory); err != nil || !info.IsDir() {
		add(semanticir.DiagnosticInvalidReference, "Rust workspace working directory is unavailable")
	}
	if len(workspace.Entries) == 0 || len(request.FocusArtifacts) == 0 {
		add(semanticir.DiagnosticInvalidInput, "Rust workspace entries and focus artifacts are required")
		return diagnostics
	}
	focused := false
	for _, artifact := range request.FocusArtifacts {
		if artifact == request.Artifact {
			focused = true
		}
	}
	if !focused {
		add(semanticir.DiagnosticInvalidReference, "Rust artifact is absent from the explicit focus artifact set")
	}
	foundSource := false
	seenPaths := make(map[string]bool, len(workspace.Entries))
	for _, entry := range workspace.Entries {
		clean := filepath.Clean(entry.Path)
		if entry.Path == "" || filepath.IsAbs(entry.Path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			add(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust workspace entry %q escapes the root", entry.Path))
			continue
		}
		if seenPaths[clean] {
			add(semanticir.DiagnosticInvalidInput, fmt.Sprintf("duplicate Rust workspace entry %q", entry.Path))
			continue
		}
		seenPaths[clean] = true
		if filepath.Clean(entry.Artifact.Path) != clean {
			add(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust workspace entry %q artifact path does not match its frozen relative path", entry.Path))
			continue
		}
		path, safe := withinRustWorkspace(workspace.Root, entry.Path)
		if !safe {
			add(semanticir.DiagnosticInvalidReference, fmt.Sprintf("Rust workspace entry %q escapes after resolution", entry.Path))
			continue
		}
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			add(semanticir.DiagnosticUnsupported, fmt.Sprintf("Rust workspace entry %q is a symlink; symlink compilation inputs are not translated", entry.Path))
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			add(semanticir.DiagnosticInvalidReference, fmt.Sprintf("read frozen Rust workspace entry %q: %v", entry.Path, err))
			continue
		}
		if err := semanticir.VerifyArtifact(entry.Artifact, content); err != nil {
			add(semanticir.DiagnosticStaleArtifact, fmt.Sprintf("Rust workspace entry %q: %v", entry.Path, err))
		}
		if entry.Artifact == request.Artifact && filepath.Clean(entry.Path) == filepath.Clean(request.Artifact.Path) {
			foundSource = true
			if !bytesEqual(content, request.Source) {
				add(semanticir.DiagnosticStaleArtifact, "Rust workspace focus bytes differ from FrontendRequest.Source")
			}
		}
	}
	if !foundSource {
		add(semanticir.DiagnosticInvalidReference, "Rust workspace has no exact entry for the focused artifact")
	}
	_ = filepath.WalkDir(workspace.Root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			add(semanticir.DiagnosticInvalidReference, "walk frozen Rust workspace: "+walkErr.Error())
			return nil
		}
		if path == workspace.Root || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(workspace.Root, path)
		if err != nil {
			add(semanticir.DiagnosticInvalidReference, "resolve Rust workspace entry: "+err.Error())
			return nil
		}
		if !seenPaths[filepath.Clean(relative)] {
			add(semanticir.DiagnosticStaleArtifact, fmt.Sprintf("Rust workspace contains unbound compilation input %q", filepath.ToSlash(relative)))
		}
		return nil
	})
	return diagnostics
}

func rustWorkspaceEnvironment(workspace semanticir.WorkspaceRef) []string {
	result := make([]string, len(workspace.Environment))
	for index, variable := range workspace.Environment {
		result[index] = variable.Name + "=" + variable.Value
	}
	return result
}

func rustWorkspaceSource(request semanticir.FrontendRequest) (string, bool) {
	for _, entry := range request.Workspace.Entries {
		if entry.Artifact == request.Artifact && filepath.Clean(entry.Path) == filepath.Clean(request.Artifact.Path) {
			return withinRustWorkspace(request.Workspace.Root, entry.Path)
		}
	}
	return "", false
}

func withinRustWorkspace(root, path string) (string, bool) {
	root = filepath.Clean(root)
	if evaluatedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = filepath.Clean(evaluatedRoot)
	}
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, filepath.Clean(path))
	}
	resolved = filepath.Clean(resolved)
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	if evaluated, err := filepath.EvalSymlinks(resolved); err == nil {
		evaluatedRelative, relErr := filepath.Rel(root, evaluated)
		if relErr != nil || evaluatedRelative == ".." || strings.HasPrefix(evaluatedRelative, ".."+string(filepath.Separator)) {
			return "", false
		}
		resolved = evaluated
	}
	return resolved, true
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
