package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/HyperMarble/hyperray/internal/taskbundle"
)

const (
	ConfigVersion     = 1
	DefaultConfigName = "hyperray.toml"
	// LegacyConfigName still loads for task folders created before the rename.
	LegacyConfigName = "hyperray.toml"
	defaultCertName  = "hyperray-certificate.json"
)

// config is deliberately private: callers select immutable task bytes, not
// injectable stage implementations. That keeps the library entry point on the
// same production path as both CLI commands.
type config struct {
	Version               int                 `toml:"version"`
	TaskID                string              `toml:"task_id"`
	SpecArtifactID        string              `toml:"spec_artifact_id"`
	InstructionArtifactID string              `toml:"instruction_artifact_id"`
	CertificatePath       string              `toml:"certificate_path"`
	Diagnostics           diagnosticsConfig   `toml:"diagnostics"`
	Freeze                freezeConfig        `toml:"freeze"`
	Translations          []translationConfig `toml:"translation"`
}

type diagnosticsConfig struct {
	PICT       pictDiagnosticConfig       `toml:"pict"`
	Oracle     commandDiagnosticConfig    `toml:"oracle"`
	DiffTest   commandDiagnosticConfig    `toml:"diff_test"`
	Dependency dependencyDiagnosticConfig `toml:"dependency"`
}

type pictDiagnosticConfig struct {
	ToolName string `toml:"tool_name"`
	Strength int    `toml:"strength"`
}

// commandDiagnosticConfig declares an exact frozen diagnostic invocation.
// Argv excludes argv[0]; the pipeline always prepends the selected frozen
// tool's absolute path and executes with the declared environment only.
type commandDiagnosticConfig struct {
	ToolName            string            `toml:"tool_name"`
	ArtifactID          string            `toml:"artifact_id"`
	WorkspacePath       string            `toml:"workspace_path"`
	SecondaryArtifactID string            `toml:"secondary_artifact_id"`
	SecondaryPath       string            `toml:"secondary_path"`
	InputsArtifactID    string            `toml:"inputs_artifact_id"`
	InputsPath          string            `toml:"inputs_path"`
	Argv                []string          `toml:"argv"`
	WorkingDirectory    string            `toml:"working_directory"`
	Environment         map[string]string `toml:"environment"`
	TimeoutMillis       int64             `toml:"timeout_millis"`
}

type dependencyDiagnosticConfig struct {
	Mode             string                  `toml:"mode"`
	Reason           string                  `toml:"reason"`
	DependencyInputs []string                `toml:"dependency_artifact_ids"`
	Run              commandDiagnosticConfig `toml:"run"`
}

type freezeConfig struct {
	Artifacts      []artifactConfig     `toml:"artifact"`
	RequiredInputs requiredInputsConfig `toml:"required_inputs"`
	Environment    environmentConfig    `toml:"environment"`
	Repository     *repositoryConfig    `toml:"repository"`
	Workspaces     []workspaceConfig    `toml:"workspace"`
}

type requiredInputsConfig struct {
	InstructionArtifactID  string   `toml:"instruction_artifact_id"`
	SpecArtifactID         string   `toml:"spec_artifact_id"`
	SolutionArtifactIDs    []string `toml:"solution_artifact_ids"`
	PublicTestArtifactIDs  []string `toml:"public_test_artifact_ids"`
	HiddenTestArtifactIDs  []string `toml:"hidden_test_artifact_ids"`
	EnvironmentArtifactIDs []string `toml:"environment_artifact_ids"`
	DependencyArtifactIDs  []string `toml:"dependency_artifact_ids"`
}

type repositoryConfig struct {
	Root          string            `toml:"root"`
	BaseCommit    string            `toml:"base_commit"`
	ToolName      string            `toml:"tool_name"`
	Environment   map[string]string `toml:"environment"`
	TimeoutMillis int64             `toml:"timeout_millis"`
}

type artifactConfig struct {
	ID   string `toml:"id"`
	Kind string `toml:"kind"`
	Path string `toml:"path"`
}

type environmentConfig struct {
	Identity      string            `toml:"identity"`
	Configuration map[string]string `toml:"configuration"`
	Tools         []toolConfig      `toml:"tool"`
}

type toolConfig struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Path        string   `toml:"path"`
	VersionArgs []string `toml:"version_args"`
}

type workspaceConfig struct {
	State      string           `toml:"state"`
	Root       string           `toml:"root"`
	Derivation derivationConfig `toml:"derivation"`
	Command    commandConfig    `toml:"command"`
}

type derivationConfig struct {
	Parent           string                  `toml:"parent"`
	Changes          []workspaceChangeConfig `toml:"change"`
	PatchArtifactIDs []string                `toml:"patch_artifact_ids"`
}

type workspaceChangeConfig struct {
	ArtifactID string `toml:"artifact_id"`
	Path       string `toml:"path"`
}

type commandConfig struct {
	Text             string            `toml:"text"`
	Shell            string            `toml:"shell"`
	ShellToolName    string            `toml:"shell_tool_name"`
	WorkingDirectory string            `toml:"working_directory"`
	Environment      map[string]string `toml:"environment"`
	TimeoutMillis    int64             `toml:"timeout_millis"`
	PassSignal       passSignalConfig  `toml:"pass_signal"`
}

type passSignalConfig struct {
	Source   string `toml:"source"`
	Match    string `toml:"match"`
	Expected string `toml:"expected"`
	Path     string `toml:"path"`
}

type translationConfig struct {
	ArtifactID                    string            `toml:"artifact_id"`
	WorkspacePath                 string            `toml:"workspace_path"`
	CompilationDatabase           string            `toml:"compilation_database"`
	ToolName                      string            `toml:"tool_name"`
	ProverToolName                string            `toml:"prover_tool_name"`
	RunnerToolName                string            `toml:"runner_tool_name"`
	RunnerConfigurationArtifactID string            `toml:"runner_configuration_artifact_id"`
	Language                      string            `toml:"language"`
	Kind                          string            `toml:"kind"`
	EntryPoints                   []string          `toml:"entry_points"`
	ObservedOperations            []string          `toml:"observed_operations"`
	Options                       map[string]string `toml:"options"`
}

func loadConfig(req Request) (config, string, []byte, error) {
	root, err := filepath.Abs(req.Root)
	if err != nil {
		return config{}, "", nil, fmt.Errorf("resolve task root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return config{}, "", nil, fmt.Errorf("task root: %w", err)
	}
	if !info.IsDir() {
		return config{}, "", nil, fmt.Errorf("task root %q is not a directory", req.Root)
	}

	path := req.ConfigPath
	if path == "" {
		path = filepath.Join(root, DefaultConfigName)
		if _, statErr := os.Stat(path); statErr != nil {
			// Old task folders still carry hyperray.toml; load it instead of
			// failing on the new name.
			legacy := filepath.Join(root, LegacyConfigName)
			if _, statErr := os.Stat(legacy); statErr == nil {
				path = legacy
			}
		}
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return config{}, "", nil, fmt.Errorf("resolve hyperray config: %w", err)
	}
	if !withinRoot(root, path) {
		return config{}, "", nil, fmt.Errorf("hyperray config %q is outside task root", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return config{}, "", nil, fmt.Errorf("read hyperray config: %w", err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	var cfg config
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, "", nil, fmt.Errorf("decode hyperray config: %w", err)
	}
	if req.CertificatePath != "" {
		cfg.CertificatePath = req.CertificatePath
	}
	if err := cfg.validate(root, path); err != nil {
		return config{}, "", nil, err
	}
	return cfg, path, content, nil
}

func (cfg config) validate(root, configPath string) error {
	if cfg.Version != ConfigVersion {
		return fmt.Errorf("hyperray config version %d, want %d", cfg.Version, ConfigVersion)
	}
	if strings.TrimSpace(cfg.TaskID) == "" {
		return errors.New("hyperray config: task_id is required")
	}
	if strings.TrimSpace(cfg.SpecArtifactID) == "" {
		return errors.New("hyperray config: spec_artifact_id is required")
	}
	if strings.TrimSpace(cfg.InstructionArtifactID) == "" {
		return errors.New("hyperray config: instruction_artifact_id is required")
	}
	if cfg.SpecArtifactID == cfg.InstructionArtifactID {
		return errors.New("hyperray config: spec and instruction must be distinct frozen artifacts")
	}
	if cfg.Freeze.RequiredInputs.SpecArtifactID != cfg.SpecArtifactID || cfg.Freeze.RequiredInputs.InstructionArtifactID != cfg.InstructionArtifactID {
		return errors.New("hyperray config: top-level spec/instruction IDs must exactly equal freeze.required_inputs role bindings")
	}
	if len(cfg.Freeze.Artifacts) == 0 {
		return errors.New("hyperray config: at least one freeze.artifact is required")
	}
	if len(cfg.Freeze.Workspaces) != 3 {
		return fmt.Errorf("hyperray config: exactly three freeze.workspace entries are required, got %d", len(cfg.Freeze.Workspaces))
	}
	toolNames := map[string]bool{}
	for _, tool := range cfg.Freeze.Environment.Tools {
		if tool.Name == "" || tool.Version == "" {
			return errors.New("hyperray config: every freeze.environment.tool requires name and version")
		}
		if toolNames[tool.Name] {
			return fmt.Errorf("hyperray config: duplicate environment tool %q", tool.Name)
		}
		toolNames[tool.Name] = true
	}
	if cfg.Freeze.Repository == nil {
		return errors.New("hyperray config: freeze.repository is required to prove every prepared workspace from an exact base commit and ordered patches")
	}
	repository := cfg.Freeze.Repository
	if repository.Root == "" || filepath.IsAbs(repository.Root) || strings.HasPrefix(filepath.Clean(repository.Root), "..") {
		return errors.New("hyperray config: freeze.repository.root must be task-relative")
	}
	if strings.TrimSpace(repository.BaseCommit) == "" || strings.TrimSpace(repository.ToolName) == "" || !toolNames[repository.ToolName] || repository.TimeoutMillis <= 0 {
		return errors.New("hyperray config: freeze.repository requires base_commit, a declared tool_name, and a positive timeout_millis")
	}
	declaredArtifactKinds := make(map[string]string, len(cfg.Freeze.Artifacts))
	for _, artifact := range cfg.Freeze.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" || strings.TrimSpace(artifact.Kind) == "" || strings.TrimSpace(artifact.Path) == "" {
			return errors.New("hyperray config: every freeze.artifact requires id, kind, and path")
		}
		if _, duplicate := declaredArtifactKinds[artifact.ID]; duplicate {
			return fmt.Errorf("hyperray config: duplicate freeze artifact %q", artifact.ID)
		}
		declaredArtifactKinds[artifact.ID] = artifact.Kind
	}
	if err := validateRequiredInputsConfig(cfg.Freeze.RequiredInputs, declaredArtifactKinds); err != nil {
		return err
	}
	if err := cfg.Diagnostics.validate(toolNames, declaredArtifactKinds); err != nil {
		return err
	}
	workspaceByState := make(map[taskbundle.WorkspaceState]workspaceConfig, len(cfg.Freeze.Workspaces))
	for _, workspace := range cfg.Freeze.Workspaces {
		state := taskbundle.WorkspaceState(workspace.State)
		if _, duplicate := workspaceByState[state]; duplicate {
			return fmt.Errorf("hyperray config: duplicate workspace state %q", workspace.State)
		}
		workspaceByState[state] = workspace
		if strings.TrimSpace(workspace.Command.ShellToolName) == "" {
			return fmt.Errorf("hyperray config: workspace %q command requires shell_tool_name", workspace.State)
		}
		if !toolNames[workspace.Command.ShellToolName] {
			return fmt.Errorf("hyperray config: workspace %q command references undeclared shell tool %q", workspace.State, workspace.Command.ShellToolName)
		}
		signal := workspace.Command.PassSignal
		if signal.Match != "exact" || (signal.Source != "exit-code" && signal.Source != "file") {
			return fmt.Errorf("hyperray config: workspace %q pass signal must be exact exit-code or file for executable confirmation", workspace.State)
		}
		if len(workspace.Derivation.Changes) != 0 {
			return fmt.Errorf("hyperray config: repository-backed workspace %q cannot use caller-asserted whole-file changes", workspace.State)
		}
		seenPatches := map[string]bool{}
		for index, artifactID := range workspace.Derivation.PatchArtifactIDs {
			_, declared := declaredArtifactKinds[artifactID]
			if artifactID == "" || !declared || seenPatches[artifactID] {
				return fmt.Errorf("hyperray config: workspace %q has an empty, undeclared, or repeated patch artifact at position %d", workspace.State, index)
			}
			seenPatches[artifactID] = true
		}
	}
	base, baseOK := workspaceByState[taskbundle.BaseOldTests]
	tests, testsOK := workspaceByState[taskbundle.BaseNewTests]
	solution, solutionOK := workspaceByState[taskbundle.SolutionNewTests]
	if !baseOK || !testsOK || !solutionOK {
		return errors.New("hyperray config: workspace triple must contain base+old-tests, base+new-tests, and base+solution+new-tests")
	}
	if base.Derivation.Parent != "" || len(base.Derivation.PatchArtifactIDs) != 0 {
		return errors.New("hyperray config: base+old-tests must be the unpatched repository base commit")
	}
	testPatchPrefix := tests.Derivation.PatchArtifactIDs
	if tests.Derivation.Parent != string(taskbundle.BaseOldTests) || len(testPatchPrefix) == 0 {
		return errors.New("hyperray config: base+new-tests must apply at least one declared test patch to base+old-tests")
	}
	for _, artifactID := range testPatchPrefix {
		if declaredArtifactKinds[artifactID] != "tests" {
			return fmt.Errorf("hyperray config: test patch %q has kind %q, want tests", artifactID, declaredArtifactKinds[artifactID])
		}
	}
	solutionPatches := solution.Derivation.PatchArtifactIDs
	if solution.Derivation.Parent != string(taskbundle.BaseNewTests) || len(solutionPatches) <= len(testPatchPrefix) || !reflect.DeepEqual(solutionPatches[:len(testPatchPrefix)], testPatchPrefix) {
		return errors.New("hyperray config: base+solution+new-tests must preserve the exact test-patch prefix and append a solution patch")
	}
	for _, artifactID := range solutionPatches[len(testPatchPrefix):] {
		kind := declaredArtifactKinds[artifactID]
		if kind != "solution" && kind != "code" {
			return fmt.Errorf("hyperray config: solution patch %q has kind %q, want solution or code", artifactID, kind)
		}
	}
	for i := 1; i < len(cfg.Freeze.Workspaces); i++ {
		if !reflect.DeepEqual(cfg.Freeze.Workspaces[0].Command, cfg.Freeze.Workspaces[i].Command) {
			return errors.New("hyperray config: all three workspace states must use the same verifier command and pass signal")
		}
	}
	if len(cfg.Translations) == 0 {
		return errors.New("hyperray config: independent code and test translations are required")
	}
	seenTranslations := map[string]bool{}
	seenWorkspacePaths := map[string]bool{}
	for i, translation := range cfg.Translations {
		if translation.ArtifactID == "" {
			return fmt.Errorf("hyperray config: translation %d has no artifact_id", i)
		}
		if translation.WorkspacePath == "" || filepath.IsAbs(translation.WorkspacePath) || strings.HasPrefix(filepath.Clean(translation.WorkspacePath), "..") {
			return fmt.Errorf("hyperray config: translation %q requires a solution-workspace-relative workspace_path", translation.ArtifactID)
		}
		cleanWorkspacePath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(translation.WorkspacePath)))
		if seenWorkspacePaths[cleanWorkspacePath] {
			return fmt.Errorf("hyperray config: multiple translations bind workspace_path %q", cleanWorkspacePath)
		}
		seenWorkspacePaths[cleanWorkspacePath] = true
		if translation.ToolName == "" {
			return fmt.Errorf("hyperray config: translation %q requires tool_name", translation.ArtifactID)
		}
		if !toolNames[translation.ToolName] {
			return fmt.Errorf("hyperray config: translation %q references undeclared tool %q", translation.ArtifactID, translation.ToolName)
		}
		if translation.ProverToolName == "" {
			return fmt.Errorf("hyperray config: translation %q requires prover_tool_name", translation.ArtifactID)
		}
		if !toolNames[translation.ProverToolName] {
			return fmt.Errorf("hyperray config: translation %q references undeclared prover tool %q", translation.ArtifactID, translation.ProverToolName)
		}
		if seenTranslations[translation.ArtifactID] {
			return fmt.Errorf("hyperray config: duplicate translation for artifact %q", translation.ArtifactID)
		}
		seenTranslations[translation.ArtifactID] = true
		if declaredArtifactKinds[translation.ArtifactID] != translation.Kind {
			return fmt.Errorf("hyperray config: translation %q relabels frozen artifact kind %q as %q", translation.ArtifactID, declaredArtifactKinds[translation.ArtifactID], translation.Kind)
		}
		switch translation.Kind {
		case "code":
			if len(translation.ObservedOperations) != 0 {
				return fmt.Errorf("hyperray config: code translation %q cannot declare observed_operations", translation.ArtifactID)
			}
			if translation.RunnerToolName != "" {
				return fmt.Errorf("hyperray config: code translation %q cannot declare runner_tool_name", translation.ArtifactID)
			}
			if translation.RunnerConfigurationArtifactID != "" {
				return fmt.Errorf("hyperray config: code translation %q cannot declare runner_configuration_artifact_id", translation.ArtifactID)
			}
		case "tests":
			if translation.RunnerToolName == "" {
				return fmt.Errorf("hyperray config: test translation %q requires runner_tool_name", translation.ArtifactID)
			}
			if !toolNames[translation.RunnerToolName] {
				return fmt.Errorf("hyperray config: test translation %q references undeclared runner tool %q", translation.ArtifactID, translation.RunnerToolName)
			}
			if translation.RunnerConfigurationArtifactID == "" {
				return fmt.Errorf("hyperray config: test translation %q requires runner_configuration_artifact_id", translation.ArtifactID)
			}
		default:
			return fmt.Errorf("hyperray config: translation %q kind must be code or tests", translation.ArtifactID)
		}
		switch translation.Language {
		case "python", "rust", "cpp":
		default:
			return fmt.Errorf("hyperray config: translation %q has unsupported language %q", translation.ArtifactID, translation.Language)
		}
		if translation.Kind == "code" && len(translation.EntryPoints) == 0 {
			return fmt.Errorf("hyperray config: code translation %q requires entry_points", translation.ArtifactID)
		}
		seenEntryPoints := map[string]bool{}
		for _, entryPoint := range translation.EntryPoints {
			if strings.TrimSpace(entryPoint) == "" || seenEntryPoints[entryPoint] {
				return fmt.Errorf("hyperray config: translation %q has an empty or duplicate entry point", translation.ArtifactID)
			}
			seenEntryPoints[entryPoint] = true
		}
		seenObserved := map[string]bool{}
		for _, operation := range translation.ObservedOperations {
			if strings.TrimSpace(operation) == "" || seenObserved[operation] {
				return fmt.Errorf("hyperray config: translation %q has an empty or duplicate observed operation", translation.ArtifactID)
			}
			seenObserved[operation] = true
		}
	}
	configRel, err := filepath.Rel(root, configPath)
	if err != nil {
		return fmt.Errorf("hyperray config: resolve config artifact: %w", err)
	}
	configRel = filepath.ToSlash(configRel)
	declaresConfig, declaresSpec, declaresInstruction := false, false, false
	declaredArtifacts := map[string]bool{}
	for _, artifact := range cfg.Freeze.Artifacts {
		declaredArtifacts[artifact.ID] = true
		if filepath.ToSlash(filepath.Clean(artifact.Path)) == configRel {
			if artifact.Kind != "configuration" {
				return errors.New("hyperray config: hyperray.toml must retain frozen artifact kind configuration")
			}
			declaresConfig = true
		}
		if artifact.ID == cfg.SpecArtifactID {
			if artifact.Kind != "spec" {
				return fmt.Errorf("hyperray config: final spec artifact %q must have kind spec", artifact.ID)
			}
			declaresSpec = true
		}
		if artifact.ID == cfg.InstructionArtifactID {
			if artifact.Kind != "instruction" {
				return fmt.Errorf("hyperray config: instruction artifact %q must have kind instruction", artifact.ID)
			}
			declaresInstruction = true
		}
	}
	if !declaresConfig {
		return errors.New("hyperray config: the hyperray.toml file must itself be a declared frozen artifact")
	}
	if !declaresSpec {
		return fmt.Errorf("hyperray config: spec artifact %q is not declared", cfg.SpecArtifactID)
	}
	if !declaresInstruction {
		return fmt.Errorf("hyperray config: instruction artifact %q is not declared", cfg.InstructionArtifactID)
	}
	for _, translation := range cfg.Translations {
		if !declaredArtifacts[translation.ArtifactID] {
			return fmt.Errorf("hyperray config: translation artifact %q is not declared", translation.ArtifactID)
		}
	}
	if cfg.CertificatePath == "" {
		cfg.CertificatePath = defaultCertName
	}
	certPath := cfg.CertificatePath
	if !filepath.IsAbs(certPath) {
		certPath = filepath.Join(root, certPath)
	}
	certPath, err = filepath.Abs(certPath)
	if err != nil || !withinRoot(root, certPath) {
		return fmt.Errorf("hyperray config: certificate_path must resolve within task root")
	}
	for _, artifact := range cfg.Freeze.Artifacts {
		artifactPath := filepath.Join(root, filepath.FromSlash(artifact.Path))
		if samePath(certPath, artifactPath) {
			return fmt.Errorf("hyperray config: certificate_path would overwrite frozen artifact %q", artifact.ID)
		}
	}
	for _, workspace := range cfg.Freeze.Workspaces {
		workspacePath := workspace.Root
		if !filepath.IsAbs(workspacePath) {
			workspacePath = filepath.Join(root, filepath.FromSlash(workspacePath))
		}
		workspacePath, _ = filepath.Abs(workspacePath)
		if withinRoot(workspacePath, certPath) {
			return fmt.Errorf("hyperray config: certificate_path is inside frozen workspace %q", workspace.State)
		}
	}
	return nil
}

func validateRequiredInputsConfig(inputs requiredInputsConfig, artifacts map[string]string) error {
	want := []struct {
		label    string
		ids      []string
		kind     string
		required bool
	}{
		{"solution_artifact_ids", inputs.SolutionArtifactIDs, "solution", true},
		{"public_test_artifact_ids", inputs.PublicTestArtifactIDs, "tests", true},
		{"hidden_test_artifact_ids", inputs.HiddenTestArtifactIDs, "tests", true},
		{"environment_artifact_ids", inputs.EnvironmentArtifactIDs, "environment", true},
		{"dependency_artifact_ids", inputs.DependencyArtifactIDs, "dependency", false},
	}
	seen := map[string]string{}
	bind := func(label, id, kind string) error {
		if id == "" || artifacts[id] != kind {
			return fmt.Errorf("hyperray config: freeze.required_inputs.%s references %q with kind %q, want %q", label, id, artifacts[id], kind)
		}
		if previous := seen[id]; previous != "" {
			return fmt.Errorf("hyperray config: frozen artifact %q is assigned to both %s and %s", id, previous, label)
		}
		seen[id] = label
		return nil
	}
	if err := bind("instruction_artifact_id", inputs.InstructionArtifactID, "instruction"); err != nil {
		return err
	}
	if err := bind("spec_artifact_id", inputs.SpecArtifactID, "spec"); err != nil {
		return err
	}
	for _, role := range want {
		if role.required && len(role.ids) == 0 {
			return fmt.Errorf("hyperray config: freeze.required_inputs.%s must not be empty", role.label)
		}
		for _, id := range role.ids {
			if err := bind(role.label, id, role.kind); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cfg diagnosticsConfig) validate(tools map[string]bool, artifacts map[string]string) error {
	if !tools[cfg.PICT.ToolName] || cfg.PICT.Strength <= 0 {
		return errors.New("hyperray config: diagnostics.pict requires a frozen tool_name and positive strength")
	}
	if err := cfg.Oracle.validate("oracle", tools, artifacts, false, false); err != nil {
		return err
	}
	if err := cfg.DiffTest.validate("diff_test", tools, artifacts, true, true); err != nil {
		return err
	}
	switch cfg.Dependency.Mode {
	case "not-applicable":
		if strings.TrimSpace(cfg.Dependency.Reason) == "" || len(cfg.Dependency.DependencyInputs) != 0 || !reflect.DeepEqual(cfg.Dependency.Run, commandDiagnosticConfig{}) {
			return errors.New("hyperray config: dependency not-applicable requires a reason, no dependency IDs, and no run command")
		}
	case "run":
		if len(cfg.Dependency.DependencyInputs) == 0 {
			return errors.New("hyperray config: dependency run requires dependency_artifact_ids")
		}
		for _, id := range cfg.Dependency.DependencyInputs {
			if artifacts[id] != "dependency" {
				return fmt.Errorf("hyperray config: dependency diagnostic artifact %q must retain kind dependency", id)
			}
		}
		if err := cfg.Dependency.Run.validate("dependency.run", tools, artifacts, false, false); err != nil {
			return err
		}
	default:
		return errors.New("hyperray config: diagnostics.dependency.mode must be run or not-applicable")
	}
	return nil
}

func (cfg commandDiagnosticConfig) validate(label string, tools map[string]bool, artifacts map[string]string, requireInputs, requireSecondary bool) error {
	if !tools[cfg.ToolName] || cfg.TimeoutMillis <= 0 {
		return fmt.Errorf("hyperray config: diagnostics.%s requires a frozen tool_name and positive timeout_millis", label)
	}
	if cfg.ArtifactID == "" || artifacts[cfg.ArtifactID] == "" {
		return fmt.Errorf("hyperray config: diagnostics.%s requires a frozen artifact_id", label)
	}
	if _, err := cleanTaskRelativePath(cfg.WorkspacePath); err != nil {
		return fmt.Errorf("hyperray config: diagnostics.%s workspace_path: %w", label, err)
	}
	if requireSecondary {
		if cfg.SecondaryArtifactID == "" || artifacts[cfg.SecondaryArtifactID] == "" {
			return fmt.Errorf("hyperray config: diagnostics.%s requires a frozen secondary_artifact_id", label)
		}
		if _, err := cleanTaskRelativePath(cfg.SecondaryPath); err != nil {
			return fmt.Errorf("hyperray config: diagnostics.%s secondary_path: %w", label, err)
		}
	} else if cfg.SecondaryArtifactID != "" || cfg.SecondaryPath != "" {
		return fmt.Errorf("hyperray config: diagnostics.%s cannot declare a secondary artifact", label)
	}
	if requireInputs {
		if cfg.InputsArtifactID == "" || artifacts[cfg.InputsArtifactID] == "" {
			return fmt.Errorf("hyperray config: diagnostics.%s requires a frozen inputs_artifact_id", label)
		}
		if _, err := cleanTaskRelativePath(cfg.InputsPath); err != nil {
			return fmt.Errorf("hyperray config: diagnostics.%s inputs_path: %w", label, err)
		}
	}
	if _, err := cleanWorkingDirectory(cfg.WorkingDirectory); err != nil {
		return fmt.Errorf("hyperray config: diagnostics.%s working_directory: %w", label, err)
	}
	if len(cfg.Argv) == 0 {
		return fmt.Errorf("hyperray config: diagnostics.%s argv must not be empty", label)
	}
	for index, argument := range cfg.Argv {
		if argument == "" || strings.ContainsRune(argument, '\x00') {
			return fmt.Errorf("hyperray config: diagnostics.%s argv[%d] is empty or contains NUL", label, index)
		}
	}
	return nil
}

func cleanTaskRelativePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || strings.ContainsRune(path, '\x00') {
		return "", errors.New("must be a nonempty workspace-relative path")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("must remain within the workspace")
	}
	return clean, nil
}

func cleanWorkingDirectory(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, '\x00') {
		return "", errors.New("must be a nonempty workspace-relative directory")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("must remain within the workspace")
	}
	return clean, nil
}

func (cfg config) freezeRequest() taskbundle.Request {
	request := taskbundle.Request{
		RequiredInputs: taskbundle.RequiredInputs{
			InstructionArtifactID:  cfg.Freeze.RequiredInputs.InstructionArtifactID,
			SpecArtifactID:         cfg.Freeze.RequiredInputs.SpecArtifactID,
			SolutionArtifactIDs:    append([]string(nil), cfg.Freeze.RequiredInputs.SolutionArtifactIDs...),
			PublicTestArtifactIDs:  append([]string(nil), cfg.Freeze.RequiredInputs.PublicTestArtifactIDs...),
			HiddenTestArtifactIDs:  append([]string(nil), cfg.Freeze.RequiredInputs.HiddenTestArtifactIDs...),
			EnvironmentArtifactIDs: append([]string(nil), cfg.Freeze.RequiredInputs.EnvironmentArtifactIDs...),
			DependencyArtifactIDs:  append([]string(nil), cfg.Freeze.RequiredInputs.DependencyArtifactIDs...),
		},
		Environment: taskbundle.Environment{
			Identity:      cfg.Freeze.Environment.Identity,
			Configuration: cloneMap(cfg.Freeze.Environment.Configuration),
		},
	}
	if cfg.Freeze.Repository != nil {
		repository := cfg.Freeze.Repository
		request.Repository = &taskbundle.RepositoryInput{
			Root: repository.Root, BaseCommit: repository.BaseCommit, ToolName: repository.ToolName,
			Environment: cloneMap(repository.Environment), TimeoutMillis: repository.TimeoutMillis,
		}
	}
	for _, artifact := range cfg.Freeze.Artifacts {
		request.Artifacts = append(request.Artifacts, taskbundle.ArtifactSpec{
			ID: artifact.ID, Kind: artifact.Kind, Path: artifact.Path,
		})
	}
	for _, tool := range cfg.Freeze.Environment.Tools {
		request.Environment.Tools = append(request.Environment.Tools, taskbundle.ToolVersion{
			Name: tool.Name, Version: tool.Version, Path: tool.Path,
			VersionArgs: append([]string(nil), tool.VersionArgs...),
		})
	}
	for _, workspace := range cfg.Freeze.Workspaces {
		request.Workspaces = append(request.Workspaces, taskbundle.WorkspaceInput{
			State: taskbundle.WorkspaceState(workspace.State),
			Root:  workspace.Root,
			Derivation: taskbundle.WorkspaceDerivation{
				Parent:           taskbundle.WorkspaceState(workspace.Derivation.Parent),
				PatchArtifactIDs: append([]string(nil), workspace.Derivation.PatchArtifactIDs...),
				Changes: func() []taskbundle.WorkspaceChange {
					changes := make([]taskbundle.WorkspaceChange, 0, len(workspace.Derivation.Changes))
					for _, change := range workspace.Derivation.Changes {
						changes = append(changes, taskbundle.WorkspaceChange{ArtifactID: change.ArtifactID, Path: change.Path})
					}
					return changes
				}(),
			},
			Command: taskbundle.Command{
				Text:             workspace.Command.Text,
				Shell:            workspace.Command.Shell,
				ShellToolName:    workspace.Command.ShellToolName,
				WorkingDirectory: workspace.Command.WorkingDirectory,
				Environment:      cloneMap(workspace.Command.Environment),
				TimeoutMillis:    workspace.Command.TimeoutMillis,
				PassSignal: taskbundle.PassSignal{
					Source:   taskbundle.SignalSource(workspace.Command.PassSignal.Source),
					Match:    taskbundle.SignalMatch(workspace.Command.PassSignal.Match),
					Expected: workspace.Command.PassSignal.Expected,
					Path:     workspace.Command.PassSignal.Path,
				},
			},
		})
	}
	return request
}

func (cfg config) certificatePath(root string) string {
	path := cfg.CertificatePath
	if path == "" {
		path = defaultCertName
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return filepath.Clean(path)
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftAbs == rightAbs
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
