package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

// frontendChangedRanges derives the exact patch phase owned by one frontend:
// test artifacts are the base-old -> base-new delta, while code artifacts are
// the base-new -> solution-new delta. This happens before translation so a
// frontend cannot use ray.toml entry-point selectors to omit changed
// declarations or impacted callers.
func frontendChangedRanges(root string, manifest taskbundle.Manifest, declaration translationConfig, artifact semanticir.ArtifactRef) ([]semanticir.ChangedSourceRange, error) {
	oldState, newState := taskbundle.BaseNewTests, taskbundle.SolutionNewTests
	if declaration.Kind == string(semanticir.ArtifactTests) {
		oldState, newState = taskbundle.BaseOldTests, taskbundle.BaseNewTests
	}
	oldWorkspace, newWorkspace, err := workspacePair(manifest, oldState, newState)
	if err != nil {
		return nil, err
	}
	path := filepath.ToSlash(filepath.Clean(declaration.WorkspacePath))
	oldEntry, oldOK := workspaceEntries(oldWorkspace)[path]
	newEntry, newOK := workspaceEntries(newWorkspace)[path]
	if !newOK || newEntry.Kind != "file" || newEntry.SHA256 != artifact.Digest {
		return nil, fmt.Errorf("translation artifact %q is not the exact regular-file %s delta target at %q", artifact.ID, newState, path)
	}
	var oldSource []byte
	if oldOK {
		if oldEntry.Kind != "file" {
			return nil, fmt.Errorf("translation artifact %q patch base at %q is not a regular file", artifact.ID, path)
		}
		oldSource, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(oldWorkspace.Path), filepath.FromSlash(path)))
		if err != nil {
			return nil, fmt.Errorf("read %s patch base %q: %w", oldState, path, err)
		}
	}
	newSource, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newWorkspace.Path), filepath.FromSlash(path)))
	if err != nil {
		return nil, fmt.Errorf("read %s patch target %q: %w", newState, path, err)
	}
	lines, err := changedSolutionLines(oldSource, newSource)
	if err != nil {
		return nil, fmt.Errorf("derive exact changed ranges for artifact %q: %w", artifact.ID, err)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("translation artifact %q is not changed in its authoritative %s -> %s patch phase", artifact.ID, oldState, newState)
	}
	provenanceFor := func(start, end int) semanticir.Provenance {
		return semanticir.NewProvenance(artifact, semanticir.SourceLocation{
			Path: artifact.Path, StartLine: start, StartColumn: 1, EndLine: end,
		}, semanticir.TranslationTranslated)
	}
	var ranges []semanticir.ChangedSourceRange
	for index := 0; index < len(lines); {
		start, end := lines[index], lines[index]
		for index++; index < len(lines) && lines[index] == end+1; index++ {
			end = lines[index]
		}
		slice, sliceErr := sourceLineSlice(newSource, start, end)
		if sliceErr != nil {
			return nil, fmt.Errorf("digest exact changed range for artifact %q: %w", artifact.ID, sliceErr)
		}
		ranges = append(ranges, semanticir.ChangedSourceRange{
			ArtifactID: artifact.ID, Path: artifact.Path, StartLine: start, EndLine: end,
			SliceDigest: semanticir.DigestBytes(slice), Provenance: provenanceFor(start, end),
		})
	}
	return ranges, nil
}

func workspacePair(manifest taskbundle.Manifest, oldState, newState taskbundle.WorkspaceState) (taskbundle.Workspace, taskbundle.Workspace, error) {
	var oldWorkspace, newWorkspace taskbundle.Workspace
	for _, workspace := range manifest.Workspaces {
		if workspace.State == oldState {
			oldWorkspace = workspace
		}
		if workspace.State == newState {
			newWorkspace = workspace
		}
	}
	if oldWorkspace.Path == "" || newWorkspace.Path == "" {
		return oldWorkspace, newWorkspace, fmt.Errorf("frozen patch scope lacks %s or %s workspace", oldState, newState)
	}
	return oldWorkspace, newWorkspace, nil
}

// sourceLineSlice returns the byte-exact complete source lines in the closed
// one-based range, retaining newline bytes. SliceDigest therefore has one
// language-independent definition that frontends can independently replay.
func sourceLineSlice(source []byte, start, end int) ([]byte, error) {
	if start <= 0 || end < start {
		return nil, fmt.Errorf("invalid line range %d-%d", start, end)
	}
	line, begin, finish := 1, -1, -1
	for index := 0; index <= len(source); index++ {
		if line == start && begin == -1 {
			begin = index
		}
		if index == len(source) || source[index] == '\n' {
			if line == end {
				finish = index
				if index < len(source) {
					finish++
				}
				break
			}
			line++
		}
	}
	if begin < 0 || finish < begin {
		return nil, fmt.Errorf("line range %d-%d exceeds %d source bytes", start, end, len(source))
	}
	return append([]byte(nil), source[begin:finish]...), nil
}

// validatePatchScope prevents ray.toml entry-point names from shrinking the
// proof boundary. Every file changed between base+new-tests and
// solution+new-tests must be an exactly translated code artifact, and every
// changed solution line must lie inside compiler-backed source scope.
func validatePatchScope(root string, manifest taskbundle.Manifest, records []translationRecord) []string {
	base, solution, err := patchWorkspaces(manifest)
	if err != nil {
		return []string{err.Error()}
	}
	baseEntries := workspaceEntries(base)
	solutionEntries := workspaceEntries(solution)
	recordsByPath := map[string]translationRecord{}
	for _, record := range records {
		if record.request.Kind == semanticir.ArtifactCode {
			recordsByPath[filepath.ToSlash(filepath.Clean(record.request.Artifact.Path))] = record
		}
	}
	paths := map[string]bool{}
	for path := range baseEntries {
		paths[path] = true
	}
	for path := range solutionEntries {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		oldEntry, oldOK := baseEntries[path]
		newEntry, newOK := solutionEntries[path]
		if oldOK && newOK && oldEntry.Kind == newEntry.Kind && oldEntry.SHA256 == newEntry.SHA256 {
			continue
		}
		record, translated := recordsByPath[path]
		if !translated {
			return []string{fmt.Sprintf("frozen solution patch changes unscoped workspace path %q", path)}
		}
		if !newOK || newEntry.Kind != "file" || (oldOK && oldEntry.Kind != "file") {
			return []string{fmt.Sprintf("changed code path %q is deleted, symlinked, or otherwise not an exact regular-file translation", path)}
		}
		oldSource := []byte{}
		if oldOK {
			oldSource, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(base.Path), filepath.FromSlash(path)))
			if err != nil {
				return []string{fmt.Sprintf("read base patch artifact %q: %v", path, err)}
			}
		}
		newSource, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(solution.Path), filepath.FromSlash(path)))
		if err != nil {
			return []string{fmt.Sprintf("read solution patch artifact %q: %v", path, err)}
		}
		if semanticir.DigestBytes(newSource) != record.request.Artifact.Digest {
			return []string{fmt.Sprintf("changed patch artifact %q differs from its frontend binding", path)}
		}
		lines, err := changedSolutionLines(oldSource, newSource)
		if err != nil {
			return []string{fmt.Sprintf("derive frozen patch scope for %q: %v", path, err)}
		}
		if err := compilerScopeCovers(record, lines); err != nil {
			return []string{fmt.Sprintf("changed patch artifact %q: %v", path, err)}
		}
	}
	return nil
}

func patchWorkspaces(manifest taskbundle.Manifest) (taskbundle.Workspace, taskbundle.Workspace, error) {
	var base, solution taskbundle.Workspace
	for _, workspace := range manifest.Workspaces {
		switch workspace.State {
		case taskbundle.BaseNewTests:
			base = workspace
		case taskbundle.SolutionNewTests:
			solution = workspace
		}
	}
	if base.Path == "" || solution.Path == "" {
		return base, solution, fmt.Errorf("frozen patch scope requires base+new-tests and solution+new-tests workspaces")
	}
	return base, solution, nil
}

func workspaceEntries(workspace taskbundle.Workspace) map[string]taskbundle.WorkspaceEntry {
	entries := make(map[string]taskbundle.WorkspaceEntry, len(workspace.Entries))
	for _, entry := range workspace.Entries {
		entries[filepath.ToSlash(filepath.Clean(entry.Path))] = entry
	}
	return entries
}

func changedSolutionLines(oldSource, newSource []byte) ([]int, error) {
	if !bytes.Equal(bytes.ToValidUTF8(oldSource, nil), oldSource) || !bytes.Equal(bytes.ToValidUTF8(newSource, nil), newSource) {
		return nil, fmt.Errorf("non-UTF-8 source cannot be line-scoped")
	}
	oldLines := splitPatchLines(string(oldSource))
	newLines := splitPatchLines(string(newSource))
	if uint64(len(oldLines)+1)*uint64(len(newLines)+1) > 16_000_000 {
		return nil, fmt.Errorf("exact line-diff resource bound exceeded")
	}
	rows := make([][]int, len(oldLines)+1)
	for i := range rows {
		rows[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				rows[i][j] = rows[i+1][j+1] + 1
			} else if rows[i+1][j] >= rows[i][j+1] {
				rows[i][j] = rows[i+1][j]
			} else {
				rows[i][j] = rows[i][j+1]
			}
		}
	}
	changed := map[int]bool{}
	for i, j := 0, 0; i < len(oldLines) || j < len(newLines); {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			i, j = i+1, j+1
		case j < len(newLines) && (i == len(oldLines) || rows[i][j+1] > rows[i+1][j]):
			changed[j+1] = true
			j++
		default:
			// A deletion has no new line. Bind it to the exact insertion
			// boundary in the solution so an omitted deleted branch cannot
			// disappear outside compiler-backed scope.
			line := j + 1
			if line > len(newLines) {
				line = len(newLines)
			}
			if line < 1 {
				line = 1
			}
			changed[line] = true
			i++
		}
	}
	result := make([]int, 0, len(changed))
	for line := range changed {
		result = append(result, line)
	}
	sort.Ints(result)
	return result, nil
}

func splitPatchLines(source string) []string {
	if source == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}

func compilerScopeCovers(record translationRecord, changedLines []int) error {
	if len(changedLines) == 0 {
		return fmt.Errorf("changed digest produced no exact changed-line scope")
	}
	closure := record.model.ScopeClosure
	if closure == nil || !closure.Complete || closure.Completeness != semanticir.ProofProved {
		return fmt.Errorf("changed source has no complete compiler-backed entrypoint/caller closure")
	}
	impacted := make(map[string]bool, len(closure.ImpactedDeclarationIDs))
	for _, declarationID := range closure.ImpactedDeclarationIDs {
		impacted[declarationID] = true
	}
	for _, line := range changedLines {
		covered := false
		for _, declaration := range closure.Declarations {
			if !declaration.Changed || !impacted[declaration.ID] || declaration.Artifact != record.request.Artifact {
				continue
			}
			location := declaration.Location
			if filepath.ToSlash(filepath.Clean(location.Path)) != filepath.ToSlash(filepath.Clean(record.request.Artifact.Path)) {
				continue
			}
			end := location.EndLine
			if end == 0 {
				end = location.StartLine
			}
			if line >= location.StartLine && line <= end {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("changed line %d is outside every compiler-backed translated entrypoint/caller scope", line)
		}
	}
	return nil
}
