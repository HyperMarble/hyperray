package cpp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

type compilationCommand struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
	Command   string   `json:"command"`
}

func (l *lowerer) validateWorkspaceCompilation() bool {
	workspace := l.request.Workspace
	valid := true
	if workspace.ID == "" || workspace.BuildCommand == "" {
		l.invalid(nil, "frozen workspace id and build command are required for C++ translation")
		valid = false
	}
	if !semanticir.ValidDigest(workspace.TreeDigest) {
		l.invalid(nil, "workspace tree digest must be normalized SHA-256")
		valid = false
	}
	if !validateCPPEnvironment(workspace) {
		l.invalid(nil, "C++ workspace lacks an exact sorted clear-environment/process-group execution contract")
		valid = false
	}
	switch workspace.State {
	case semanticir.WorkspaceBaseOldTests, semanticir.WorkspaceBaseNewTests, semanticir.WorkspaceSolutionNewTests:
	default:
		l.invalid(nil, fmt.Sprintf("workspace state %q is invalid", workspace.State))
		valid = false
	}
	if !filepath.IsAbs(workspace.Root) {
		l.invalid(nil, "workspace root must be absolute")
		return false
	}
	rootInfo, err := os.Stat(workspace.Root)
	if err != nil || !rootInfo.IsDir() {
		l.invalid(nil, fmt.Sprintf("workspace root %q is unavailable or not a directory", workspace.Root))
		return false
	}
	workingDirectory, ok := withinWorkspace(workspace.Root, workspace.WorkingDirectory)
	if !ok {
		l.invalid(nil, "workspace working directory must resolve within the frozen workspace root")
		valid = false
	} else if info, err := os.Stat(workingDirectory); err != nil || !info.IsDir() {
		l.invalid(nil, fmt.Sprintf("workspace working directory %q is unavailable", workingDirectory))
		valid = false
	}

	focused := false
	focusSet := make(map[semanticir.ArtifactRef]bool, len(l.request.FocusArtifacts))
	for _, artifact := range l.request.FocusArtifacts {
		focusSet[artifact] = false
		if artifact == l.request.Artifact {
			focused = true
		}
	}
	if !focused {
		l.invalid(nil, "frontend artifact is absent from the exact focus artifact set")
		valid = false
	}

	var sourcePath string
	var compilationBytes []byte
	seenPaths := make(map[string]bool, len(workspace.Entries))
	for _, entry := range workspace.Entries {
		cleanEntry := filepath.Clean(entry.Path)
		if entry.Path == "" || filepath.IsAbs(entry.Path) || cleanEntry == ".." || strings.HasPrefix(cleanEntry, ".."+string(filepath.Separator)) {
			l.invalid(nil, fmt.Sprintf("workspace entry %q must be a root-relative path", entry.Path))
			valid = false
			continue
		}
		if seenPaths[cleanEntry] {
			l.invalid(nil, fmt.Sprintf("duplicate workspace entry path %q", entry.Path))
			valid = false
			continue
		}
		seenPaths[cleanEntry] = true
		if filepath.Clean(entry.Artifact.Path) != cleanEntry {
			l.invalid(nil, fmt.Sprintf("workspace entry %q does not exactly bind artifact path %q", entry.Path, entry.Artifact.Path))
			valid = false
		}
		path, safe := withinWorkspace(workspace.Root, entry.Path)
		if !safe {
			l.invalid(nil, fmt.Sprintf("workspace entry path %q escapes the root", entry.Path))
			valid = false
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			l.invalid(nil, fmt.Sprintf("read frozen workspace entry %q: %v", entry.Path, err))
			valid = false
			continue
		}
		if err := semanticir.VerifyArtifact(entry.Artifact, content); err != nil {
			l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("workspace entry %q: %v", entry.Path, err))
			valid = false
		}
		if entry.Artifact == l.request.Artifact && filepath.Clean(entry.Path) == filepath.Clean(l.request.Artifact.Path) {
			sourcePath = path
			if string(content) != string(l.request.Source) {
				l.diagnostic(nil, semanticir.DiagnosticStaleArtifact, "workspace focus source bytes differ from FrontendRequest.Source")
				valid = false
			}
		}
		if _, isFocus := focusSet[entry.Artifact]; isFocus {
			focusSet[entry.Artifact] = true
		}
		if workspace.CompilationDatabase != nil && entry.Artifact == *workspace.CompilationDatabase {
			compilationBytes = content
		}
	}
	if sourcePath == "" {
		l.invalid(nil, "frozen workspace has no exact entry for the focus artifact")
		valid = false
	}
	for artifact, found := range focusSet {
		if !found {
			l.invalid(nil, fmt.Sprintf("focus artifact %q has no exact frozen workspace entry", artifact.ID))
			valid = false
		}
	}
	if workspace.CompilationDatabase == nil {
		l.invalid(nil, "C++ translation requires a frozen compilation database artifact")
		return false
	}
	if len(compilationBytes) == 0 {
		l.invalid(nil, "workspace compilation database artifact is absent from frozen entries")
		return false
	}
	if !valid {
		return false
	}

	var commands []compilationCommand
	if err := json.Unmarshal(compilationBytes, &commands); err != nil {
		l.invalid(nil, fmt.Sprintf("parse frozen compilation database: %v", err))
		return false
	}
	var matches []compilationCommand
	for _, command := range commands {
		directory := command.Directory
		if directory == "" {
			directory = workingDirectory
		} else if !filepath.IsAbs(directory) {
			directory = filepath.Join(workspace.Root, directory)
		}
		resolvedDirectory, inside := withinWorkspace(workspace.Root, directory)
		if !inside {
			continue
		}
		directory = resolvedDirectory
		file := command.File
		if !filepath.IsAbs(file) {
			file = filepath.Join(directory, file)
		}
		if samePath(file, sourcePath) {
			command.Directory = directory
			matches = append(matches, command)
		}
	}
	if len(matches) != 1 {
		l.invalid(nil, fmt.Sprintf("compilation database must contain exactly one command for focus source %q, found %d", l.request.Artifact.Path, len(matches)))
		return false
	}
	command := matches[0]
	if len(command.Arguments) == 0 {
		if command.Command != "" {
			l.block(nil, "shell-compile-command", "compilation database command-only entry cannot be tokenized without guessing shell semantics")
		} else {
			l.invalid(nil, "compilation database entry has neither arguments nor command")
		}
		return false
	}
	flags, err := normalizedCompilationArguments(command, sourcePath, l.request.Translator.Path, workspace)
	if err != nil {
		l.invalid(nil, err.Error())
		return false
	}
	if err := validateCompileFlags(flags); err != nil {
		l.invalid(nil, err.Error())
		return false
	}
	l.compileDirectory = command.Directory
	l.compileFlags = flags
	l.sourcePath = sourcePath
	return true
}

func normalizedCompilationArguments(command compilationCommand, sourcePath, translatorPath string, workspace semanticir.WorkspaceRef) ([]string, error) {
	if len(command.Arguments) < 2 {
		return nil, fmt.Errorf("compilation database arguments do not include compiler and focus input")
	}
	compiler := command.Arguments[0]
	if !filepath.IsAbs(compiler) {
		if filepath.Base(compiler) != compiler {
			return nil, fmt.Errorf("compilation database compiler %q is a relative path rather than a frozen PATH tool name", compiler)
		}
		resolved, ok := workspaceLookPath(compiler, workspace)
		if !ok {
			return nil, fmt.Errorf("compilation database compiler %q is absent from the frozen workspace PATH", compiler)
		}
		compiler = resolved
	}
	if !sameExecutable(compiler, translatorPath) {
		return nil, fmt.Errorf("compilation database compiler %q does not match pinned translator %q", command.Arguments[0], translatorPath)
	}
	foundInput := false
	flags := make([]string, 0, len(command.Arguments))
	for index := 1; index < len(command.Arguments); index++ {
		argument := command.Arguments[index]
		candidate := argument
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(command.Directory, candidate)
		}
		if samePath(candidate, sourcePath) {
			foundInput = true
			continue
		}
		switch argument {
		case "-c", "-S", "-E", "-fsyntax-only", "-MD", "-MMD", "-MP":
			continue
		case "-o", "--output", "-MF", "-MT", "-MQ", "-MJ":
			if index+1 >= len(command.Arguments) {
				return nil, fmt.Errorf("compile action flag %q is missing its value", argument)
			}
			index++
			continue
		}
		if !strings.HasPrefix(argument, "-") && !previousTakesValue(command.Arguments, index) {
			return nil, fmt.Errorf("compilation database contains additional uncontrolled input %q", argument)
		}
		flags = append(flags, argument)
	}
	if !foundInput {
		return nil, fmt.Errorf("compilation database arguments do not contain the exact focus input")
	}
	return flags, nil
}

func previousTakesValue(arguments []string, index int) bool {
	if index <= 0 {
		return false
	}
	switch arguments[index-1] {
	case "-I", "-isystem", "-include", "-iquote", "-D", "-U", "-target", "--target", "-isysroot", "--sysroot":
		return true
	default:
		return false
	}
}

func withinWorkspace(root, path string) (string, bool) {
	root = filepath.Clean(root)
	comparisonRoot := root
	if evaluatedRoot, err := filepath.EvalSymlinks(root); err == nil {
		comparisonRoot = evaluatedRoot
	}
	if path == "" {
		path = "."
	}
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}
	resolved = filepath.Clean(resolved)
	if evaluated, err := filepath.EvalSymlinks(resolved); err == nil {
		evaluatedRelative, relErr := filepath.Rel(comparisonRoot, evaluated)
		if relErr != nil || evaluatedRelative == ".." || strings.HasPrefix(evaluatedRelative, ".."+string(filepath.Separator)) {
			return "", false
		}
		resolved = evaluated
		return resolved, true
	}
	// A non-existent target cannot be canonicalized. It is accepted only when
	// its lexical path is contained by the same raw workspace coordinate
	// system; callers that need alias safety use existing parent directories.
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return resolved, true
}

func samePath(left, right string) bool {
	leftEval, leftErr := filepath.EvalSymlinks(filepath.Clean(left))
	rightEval, rightErr := filepath.EvalSymlinks(filepath.Clean(right))
	if leftErr == nil && rightErr == nil {
		return leftEval == rightEval
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func sameExecutable(left, right string) bool {
	return samePath(left, right)
}
