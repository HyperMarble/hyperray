package taskbundle

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const maxRepositoryArchiveBytes = 256 << 20

// freezeRepository resolves the declared immutable commit and exports its
// exact tree with the frozen VCS/apply tool. The returned archive is ephemeral
// execution input; its digest and extracted tree digest are retained.
func freezeRepository(ctx context.Context, taskRoot string, input *RepositoryInput, tools []ToolVersion) (*Repository, []byte, error) {
	if input == nil {
		return nil, nil, nil
	}
	full, relative, err := resolveWithin(taskRoot, input.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("freeze repository: %w", err)
	}
	if err := ensureResolvedWithin(taskRoot, full); err != nil {
		return nil, nil, fmt.Errorf("freeze repository: %w", err)
	}
	info, err := os.Stat(full)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return nil, nil, fmt.Errorf("freeze repository %q: %w", relative, err)
	}
	baseCommit := strings.TrimSpace(input.BaseCommit)
	if !validCommitID(baseCommit) {
		return nil, nil, errors.New("freeze repository: base_commit must be an exact lowercase 40- or 64-hex commit id")
	}
	toolName := strings.TrimSpace(input.ToolName)
	var tool ToolVersion
	for _, candidate := range tools {
		if candidate.Name == toolName {
			tool = candidate
			break
		}
	}
	if tool.Name == "" {
		return nil, nil, fmt.Errorf("freeze repository: tool %q is not frozen", toolName)
	}
	environment, err := normalizeDerivationEnvironment(input.Environment)
	if err != nil {
		return nil, nil, fmt.Errorf("freeze repository: %w", err)
	}
	timeoutMillis := input.TimeoutMillis
	if timeoutMillis == 0 {
		timeoutMillis = 30_000
	}
	if timeoutMillis < 1 {
		return nil, nil, errors.New("freeze repository: timeout_millis must be positive")
	}
	resolveArgv := []string{tool.Path, "-C", full, "rev-parse", "--verify", baseCommit + "^{commit}"}
	resolved, resolveStderr, err := runDerivationCommand(ctx, timeoutMillis, environment, "", resolveArgv, maxCapturedCommandOutput)
	if err != nil {
		return nil, nil, fmt.Errorf("freeze repository resolve base commit: %w", err)
	}
	if strings.TrimSpace(string(resolved)) != baseCommit {
		return nil, nil, fmt.Errorf("freeze repository: resolved commit %q differs from declared base %q", strings.TrimSpace(string(resolved)), baseCommit)
	}
	archiveArgv := []string{tool.Path, "-C", full, "archive", "--format=tar", baseCommit}
	archive, archiveStderr, err := runDerivationCommand(ctx, timeoutMillis, environment, "", archiveArgv, maxRepositoryArchiveBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("freeze repository archive base commit: %w", err)
	}
	temp, err := os.MkdirTemp("", "ray-base-tree-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(temp)
	if err := extractRepositoryArchive(archive, temp); err != nil {
		return nil, nil, fmt.Errorf("freeze repository archive: %w", err)
	}
	_, baseTreeDigest, err := snapshotTree(temp)
	if err != nil {
		return nil, nil, fmt.Errorf("freeze repository base tree: %w", err)
	}
	repository := &Repository{
		Path: relative, BaseCommit: baseCommit, BaseTreeSHA256: baseTreeDigest, ArchiveSHA256: digestBytes(archive), Tool: tool,
		ResolveArgv: resolveArgv, ArchiveArgv: archiveArgv, Environment: environment, EnvironmentSHA256: EnvironmentDigest(environment), TimeoutMillis: timeoutMillis,
		ResolveStdoutSHA256: digestBytes(resolved), ResolveStderrSHA256: digestBytes(resolveStderr), ArchiveStderrSHA256: digestBytes(archiveStderr),
	}
	return repository, archive, nil
}

func replayWorkspacePatches(ctx context.Context, taskRoot string, archive []byte, repository Repository, state WorkspaceState, patchIDs []string, artifacts []Artifact, requiredInputs RequiredInputs) (PatchReplay, error) {
	temp, err := os.MkdirTemp("", "ray-patch-replay-*")
	if err != nil {
		return PatchReplay{}, err
	}
	defer os.RemoveAll(temp)
	if err := extractRepositoryArchive(archive, temp); err != nil {
		return PatchReplay{}, err
	}
	_, initialDigest, err := snapshotTree(temp)
	if err != nil || initialDigest != repository.BaseTreeSHA256 {
		return PatchReplay{}, errors.New("archived base tree differs from frozen repository identity")
	}
	byID := make(map[string]Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	replay := PatchReplay{BaseCommit: repository.BaseCommit, BaseTreeSHA256: repository.BaseTreeSHA256}
	for _, patchID := range patchIDs {
		artifact, exists := byID[patchID]
		if !exists {
			return PatchReplay{}, fmt.Errorf("patch artifact %q is not frozen", patchID)
		}
		patchPath, _, err := resolveWithin(taskRoot, artifact.Path)
		if err != nil {
			return PatchReplay{}, err
		}
		if err := ensureResolvedWithin(taskRoot, patchPath); err != nil {
			return PatchReplay{}, err
		}
		patchBytes, err := os.ReadFile(patchPath)
		if err != nil {
			return PatchReplay{}, fmt.Errorf("read patch %q: %w", patchID, err)
		}
		if int64(len(patchBytes)) != artifact.Size || digestBytes(patchBytes) != artifact.SHA256 {
			return PatchReplay{}, fmt.Errorf("patch artifact %q changed after it was frozen", patchID)
		}
		appliedBytes, roleRanges, err := patchInputForState(patchBytes, artifact, state, requiredInputs)
		if err != nil {
			return PatchReplay{}, fmt.Errorf("select patch %q: %w", patchID, err)
		}
		argv := []string{repository.Tool.Path, "apply", "--whitespace=nowarn", "-"}
		stdout, stderr, err := runDerivationCommandInput(ctx, repository.TimeoutMillis, repository.Environment, temp, argv, appliedBytes, maxCapturedCommandOutput)
		if err != nil {
			return PatchReplay{}, fmt.Errorf("apply patch %q: %w", patchID, err)
		}
		_, resultDigest, err := snapshotTree(temp)
		if err != nil {
			return PatchReplay{}, err
		}
		replay.Applications = append(replay.Applications, PatchApplication{
			ArtifactID: artifact.ID, ArtifactKind: artifact.Kind, ArtifactPath: artifact.Path, ArtifactSHA256: artifact.SHA256,
			InputSHA256: digestBytes(appliedBytes), RoleRanges: roleRanges,
			Tool: repository.Tool, Argv: argv, Environment: cloneMap(repository.Environment), EnvironmentSHA256: repository.EnvironmentSHA256,
			TimeoutMillis: repository.TimeoutMillis, StdoutSHA256: digestBytes(stdout), StderrSHA256: digestBytes(stderr), ResultTreeSHA256: resultDigest,
		})
	}
	_, replay.ResultTreeSHA256, err = snapshotTree(temp)
	return replay, err
}

func patchInputForState(content []byte, artifact Artifact, state WorkspaceState, required RequiredInputs) ([]byte, []RoleRange, error) {
	if artifact.Kind != "patch" {
		return content, nil, nil
	}
	allowed := map[string]bool{}
	switch state {
	case BaseNewTests:
		allowed["public-test"], allowed["hidden-test"] = true, true
	case SolutionNewTests:
		allowed["public-test"], allowed["hidden-test"], allowed["solution"] = true, true, true
	default:
		return nil, nil, errors.New("mixed patch cannot be applied to the base workspace")
	}
	var selected []RoleRange
	for _, roleRange := range required.RoleRanges {
		if roleRange.ArtifactID == artifact.ID && allowed[roleRange.Role] {
			selected = append(selected, roleRange)
		}
	}
	if len(selected) == 0 {
		return nil, nil, fmt.Errorf("no %s role ranges", state)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].StartByte < selected[j].StartByte })
	var result []byte
	for _, roleRange := range selected {
		if roleRange.StartByte < 0 || roleRange.EndByte > int64(len(content)) || roleRange.EndByte <= roleRange.StartByte || digestBytes(content[roleRange.StartByte:roleRange.EndByte]) != roleRange.SHA256 {
			return nil, nil, errors.New("role range bytes differ from frozen digest")
		}
		result = append(result, content[roleRange.StartByte:roleRange.EndByte]...)
	}
	return result, selected, nil
}

func validateRepositoryBindings(manifest Manifest) error {
	if manifest.Repository == nil {
		for _, workspace := range manifest.Workspaces {
			if workspace.PatchReplay != nil || len(workspace.Derivation.PatchArtifactIDs) != 0 {
				return errors.New("manifest has patch replay evidence without a frozen repository")
			}
		}
		return nil
	}
	repository := *manifest.Repository
	cleanPath, err := cleanRelative(repository.Path)
	if err != nil || cleanPath != repository.Path || !validCommitID(repository.BaseCommit) || !validDigest(repository.BaseTreeSHA256) || !validDigest(repository.ArchiveSHA256) ||
		!validDigest(repository.ResolveStdoutSHA256) || !validDigest(repository.ResolveStderrSHA256) || !validDigest(repository.ArchiveStderrSHA256) || repository.TimeoutMillis < 1 {
		return errors.New("manifest repository identity or result digests are invalid")
	}
	frozenTool, exists := manifest.Tool(repository.Tool.Name)
	if !exists || !reflect.DeepEqual(frozenTool, repository.Tool) {
		return errors.New("manifest repository tool differs from the frozen environment")
	}
	environment, err := normalizeDerivationEnvironment(repository.Environment)
	if err != nil || !reflect.DeepEqual(environment, repository.Environment) || repository.EnvironmentSHA256 != EnvironmentDigest(environment) {
		return errors.New("manifest repository environment is not exact and canonical")
	}
	if len(repository.ResolveArgv) != 6 || repository.ResolveArgv[0] != repository.Tool.Path || repository.ResolveArgv[1] != "-C" ||
		repository.ResolveArgv[3] != "rev-parse" || repository.ResolveArgv[4] != "--verify" || repository.ResolveArgv[5] != repository.BaseCommit+"^{commit}" || !filepath.IsAbs(repository.ResolveArgv[2]) {
		return errors.New("manifest repository resolve argv is not the exact frozen invocation")
	}
	if len(repository.ArchiveArgv) != 6 || repository.ArchiveArgv[0] != repository.Tool.Path || repository.ArchiveArgv[1] != "-C" ||
		repository.ArchiveArgv[2] != repository.ResolveArgv[2] || repository.ArchiveArgv[3] != "archive" || repository.ArchiveArgv[4] != "--format=tar" || repository.ArchiveArgv[5] != repository.BaseCommit {
		return errors.New("manifest repository archive argv is not the exact frozen invocation")
	}
	artifacts := make(map[string]Artifact, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		artifacts[artifact.ID] = artifact
	}
	for _, workspace := range manifest.Workspaces {
		replay := workspace.PatchReplay
		if replay == nil || replay.BaseCommit != repository.BaseCommit || replay.BaseTreeSHA256 != repository.BaseTreeSHA256 || replay.ResultTreeSHA256 != workspace.TreeSHA256 || len(replay.Applications) != len(workspace.Derivation.PatchArtifactIDs) {
			return fmt.Errorf("manifest workspace %q patch replay is missing, truncated, or detached", workspace.State)
		}
		if len(replay.Applications) == 0 && replay.ResultTreeSHA256 != repository.BaseTreeSHA256 {
			return fmt.Errorf("manifest base workspace differs from the archived base commit")
		}
		for index, application := range replay.Applications {
			artifact := artifacts[workspace.Derivation.PatchArtifactIDs[index]]
			if artifact.ID == "" || application.ArtifactID != artifact.ID || application.ArtifactKind != artifact.Kind || application.ArtifactPath != artifact.Path || application.ArtifactSHA256 != artifact.SHA256 || !validDigest(application.InputSHA256) ||
				!reflect.DeepEqual(application.Tool, repository.Tool) || !reflect.DeepEqual(application.Environment, repository.Environment) || application.EnvironmentSHA256 != repository.EnvironmentSHA256 ||
				application.TimeoutMillis != repository.TimeoutMillis || !validDigest(application.StdoutSHA256) || !validDigest(application.StderrSHA256) || !validDigest(application.ResultTreeSHA256) {
				return fmt.Errorf("manifest workspace %q patch application %d is stale or relabeled", workspace.State, index)
			}
			if len(application.Argv) != 4 || application.Argv[0] != repository.Tool.Path || application.Argv[1] != "apply" || application.Argv[2] != "--whitespace=nowarn" || application.Argv[3] != "-" {
				return fmt.Errorf("manifest workspace %q patch application %d argv is not exact", workspace.State, index)
			}
			if artifact.Kind == "patch" {
				if len(application.RoleRanges) == 0 {
					return fmt.Errorf("manifest workspace %q mixed patch application %d omits role ranges", workspace.State, index)
				}
				for _, roleRange := range application.RoleRanges {
					found := false
					for _, frozen := range manifest.RequiredInputs.RoleRanges {
						found = found || roleRange == frozen
					}
					if !found {
						return fmt.Errorf("manifest workspace %q mixed patch application %d has a detached role range", workspace.State, index)
					}
				}
			} else if len(application.RoleRanges) != 0 || application.InputSHA256 != artifact.SHA256 {
				return fmt.Errorf("manifest workspace %q whole patch application %d has altered input bytes", workspace.State, index)
			}
		}
		if len(replay.Applications) != 0 && replay.Applications[len(replay.Applications)-1].ResultTreeSHA256 != replay.ResultTreeSHA256 {
			return fmt.Errorf("manifest workspace %q patch replay result omits its final application", workspace.State)
		}
	}
	return nil
}

func verifyRepositoryCurrent(ctx context.Context, root string, manifest Manifest) error {
	if manifest.Repository == nil {
		return nil
	}
	want := manifest.Repository
	observed, archive, err := freezeRepository(ctx, root, &RepositoryInput{
		Root: want.Path, BaseCommit: want.BaseCommit, ToolName: want.Tool.Name,
		Environment: want.Environment, TimeoutMillis: want.TimeoutMillis,
	}, manifest.Environment.Tools)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(*observed, *want) {
		return errors.New("frozen repository commit, archive, command, or base tree changed")
	}
	for _, workspace := range manifest.Workspaces {
		replay, err := replayWorkspacePatches(ctx, root, archive, *want, workspace.State, workspace.Derivation.PatchArtifactIDs, manifest.Artifacts, manifest.RequiredInputs)
		if err != nil {
			return fmt.Errorf("verify workspace %q patch replay: %w", workspace.State, err)
		}
		if workspace.PatchReplay == nil || !reflect.DeepEqual(replay, *workspace.PatchReplay) || replay.ResultTreeSHA256 != workspace.TreeSHA256 {
			return fmt.Errorf("verify workspace %q: base+ordered-patch replay changed", workspace.State)
		}
	}
	return nil
}

func runDerivationCommand(parent context.Context, timeoutMillis int64, environment map[string]string, dir string, argv []string, outputLimit int) ([]byte, []byte, error) {
	return runDerivationCommandInput(parent, timeoutMillis, environment, dir, argv, nil, outputLimit)
}

func runDerivationCommandInput(parent context.Context, timeoutMillis int64, environment map[string]string, dir string, argv []string, input []byte, outputLimit int) ([]byte, []byte, error) {
	if len(argv) == 0 || timeoutMillis < 1 {
		return nil, nil, errors.New("invalid derivation command")
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutMillis)*time.Millisecond)
	defer cancel()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, exactEnvironment(environment)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	stdout, stderr := newLimitedBuffer(outputLimit), newLimitedBuffer(maxCapturedCommandOutput)
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = stdout, stderr, 2*time.Second
	err := runManagedProcess(ctx, cmd)
	if ctx.Err() != nil {
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("command timed out after %dms: %w", timeoutMillis, ctx.Err())
	}
	if stdout.exceeded || stderr.exceeded {
		return stdout.Bytes(), stderr.Bytes(), errors.New("derivation command output exceeds evidence limit")
	}
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("command failed: %w: %s", err, strings.TrimSpace(string(stderr.Bytes())))
	}
	return append([]byte(nil), stdout.Bytes()...), append([]byte(nil), stderr.Bytes()...), nil
}

func normalizeDerivationEnvironment(environment map[string]string) (map[string]string, error) {
	result := cloneMap(environment)
	if result == nil {
		result = map[string]string{}
	}
	if _, exists := result["PATH"]; !exists {
		result["PATH"] = ""
	}
	for key, value := range result {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("invalid derivation environment variable %q", key)
		}
	}
	return result, nil
}

func validCommitID(value string) bool {
	if len(value) != 40 && len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func extractRepositoryArchive(data []byte, root string) error {
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		clean, err := cleanRelative(filepath.ToSlash(header.Name))
		if err != nil {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		target := filepath.Join(root, filepath.FromSlash(clean))
		if err := safeArchiveParents(root, filepath.Dir(target)); err != nil {
			return err
		}
		mode := os.FileMode(header.Mode).Perm()
		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// Metadata-only PAX records carry no repository path bytes. The
			// archive reader applies their key/value overrides to later files.
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			written, copyErr := io.Copy(file, io.LimitReader(reader, header.Size+1))
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil || written != header.Size {
				if copyErr != nil {
					return copyErr
				}
				if closeErr != nil {
					return closeErr
				}
				return fmt.Errorf("archive file %q size mismatch", clean)
			}
		case tar.TypeSymlink:
			link := filepath.FromSlash(header.Linkname)
			if filepath.IsAbs(link) {
				return fmt.Errorf("archive symlink %q has absolute target", clean)
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(target), link))
			relative, relErr := filepath.Rel(root, resolved)
			if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return fmt.Errorf("archive symlink %q escapes base tree", clean)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive path %q has unsupported type %d", clean, header.Typeflag)
		}
	}
}

func safeArchiveParents(root, parent string) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("archive parent escapes base tree")
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive parent %q is not a real directory", component)
		}
	}
	return nil
}
