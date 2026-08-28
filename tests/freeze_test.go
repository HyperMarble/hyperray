package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/ray/internal/taskbundle"
)

func freezeTestDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func freezeFixture(t *testing.T) (string, taskbundle.Request) {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, root, "instruction.md", "make the verifier exact\n", 0o644)
	writeFixtureFile(t, root, "spec.md", "# frozen contract\n", 0o644)
	writeFixtureFile(t, root, "tests/public.test", "public verifier source\n", 0o644)
	writeFixtureFile(t, root, "tests/hidden.test", "hidden verifier source\n", 0o644)
	writeFixtureFile(t, root, "environment/image.json", "{\"image\":\"fixture@sha256:0123\"}\n", 0o644)
	writeFixtureFile(t, root, "dependencies/lock.txt", "fixture-dependency==1.0 --hash=sha256:abcd\n", 0o644)
	writeFixtureFile(t, root, "tools/fixture-tool", "#!/bin/sh\nprintf 'fixture-tool-v1\\nbuild fixture-42\\n'\n", 0o755)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for reproducible workspace freeze tests")
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := filepath.Join(root, "repository")
	if err := os.MkdirAll(repositoryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "repository/source.py", "def answer(): return 42\n", 0o644)
	writeFixtureFile(t, root, "repository/verdict.txt", "1\n", 0o644)
	writeFixtureFile(t, root, "repository/reward.txt", "stale\n", 0o644)
	runGit := func(arguments ...string) string {
		command := exec.Command(gitPath, arguments...)
		command.Dir = repositoryRoot
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		output, runErr := command.CombinedOutput()
		if runErr != nil {
			t.Fatalf("git %v: %v: %s", arguments, runErr, output)
		}
		return strings.TrimSpace(string(output))
	}
	runGit("init", "--quiet")
	runGit("config", "user.email", "freeze@example.invalid")
	runGit("config", "user.name", "Freeze Fixture")
	runGit("add", "--", "source.py", "verdict.txt", "reward.txt")
	runGit("commit", "--quiet", "-m", "base")
	baseCommit := runGit("rev-parse", "HEAD")
	testsPatch := "diff --git a/verdict.txt b/verdict.txt\n--- a/verdict.txt\n+++ b/verdict.txt\n@@ -1 +1 @@\n-1\n+0\n"
	solutionPatch := "diff --git a/verdict.txt b/verdict.txt\n--- a/verdict.txt\n+++ b/verdict.txt\n@@ -1 +1 @@\n-0\n+1\n"
	writeFixtureFile(t, root, "tests/test.patch", testsPatch, 0o644)
	writeFixtureFile(t, root, "solution/solution.patch", solutionPatch, 0o644)

	states := []struct {
		state   taskbundle.WorkspaceState
		root    string
		verdict string
	}{
		{taskbundle.BaseOldTests, "workspaces/base-old", "1\n"},
		{taskbundle.BaseNewTests, "workspaces/base-new", "0\n"},
		{taskbundle.SolutionNewTests, "workspaces/solution-new", "1\n"},
	}
	workspaces := make([]taskbundle.WorkspaceInput, 0, len(states))
	for _, state := range states {
		writeFixtureFile(t, root, filepath.Join(state.root, "verdict.txt"), state.verdict, 0o644)
		writeFixtureFile(t, root, filepath.Join(state.root, "source.py"), "def answer(): return 42\n", 0o644)
		writeFixtureFile(t, root, filepath.Join(state.root, "reward.txt"), "stale\n", 0o644)
		workspace := taskbundle.WorkspaceInput{
			State: state.state,
			Root:  state.root,
			Command: taskbundle.Command{
				// Every command exits zero. The base+new workspace must still be
				// observed as failing through the task-declared reward file.
				Text:          "cp verdict.txt reward.txt; exit 0",
				Environment:   map[string]string{"PATH": "/bin:/usr/bin"},
				TimeoutMillis: 5_000,
				PassSignal: taskbundle.PassSignal{
					Source: taskbundle.SignalFile, Match: taskbundle.MatchExact,
					Expected: "1\n", Path: "reward.txt",
				},
			},
		}
		switch state.state {
		case taskbundle.BaseNewTests:
			workspace.Derivation = taskbundle.WorkspaceDerivation{Parent: taskbundle.BaseOldTests, PatchArtifactIDs: []string{"tests-patch"}}
		case taskbundle.SolutionNewTests:
			workspace.Derivation = taskbundle.WorkspaceDerivation{Parent: taskbundle.BaseNewTests, PatchArtifactIDs: []string{"tests-patch", "solution-patch"}}
		}
		workspaces = append(workspaces, workspace)
	}
	return root, taskbundle.Request{
		Artifacts: []taskbundle.ArtifactSpec{
			{ID: "spec", Kind: "spec", Path: "spec.md"},
			{ID: "instruction", Kind: "instruction", Path: "instruction.md"},
			{ID: "tests-patch", Kind: "tests", Path: "tests/test.patch"},
			{ID: "public-test", Kind: "tests", Path: "tests/public.test"},
			{ID: "hidden-test", Kind: "tests", Path: "tests/hidden.test"},
			{ID: "solution-patch", Kind: "solution", Path: "solution/solution.patch"},
			{ID: "environment", Kind: "environment", Path: "environment/image.json"},
			{ID: "dependencies", Kind: "dependency", Path: "dependencies/lock.txt"},
		},
		RequiredInputs: taskbundle.RequiredInputs{
			InstructionArtifactID: "instruction", SpecArtifactID: "spec",
			SolutionArtifactIDs: []string{"solution-patch"}, PublicTestArtifactIDs: []string{"public-test", "tests-patch"},
			HiddenTestArtifactIDs: []string{"hidden-test"}, EnvironmentArtifactIDs: []string{"environment"}, DependencyArtifactIDs: []string{"dependencies"},
		},
		Environment: taskbundle.Environment{
			Identity:      "docker-image@sha256:0123456789abcdef",
			Configuration: map[string]string{"LANG": "C.UTF-8", "TZ": "UTC"},
			Tools: []taskbundle.ToolVersion{
				{Name: "fixture-tool", Version: "fixture-tool-v1", Path: filepath.Join(root, "tools", "fixture-tool"), VersionArgs: []string{}},
				{Name: "shell", Version: "ray-fixture-shell", Path: "/bin/sh", VersionArgs: []string{"-c", "printf ray-fixture-shell"}},
				{Name: "git", Version: "git version", Path: gitPath, VersionArgs: []string{"--version"}},
			},
		},
		Repository: &taskbundle.RepositoryInput{Root: "repository", BaseCommit: baseCommit, ToolName: "git", Environment: map[string]string{"PATH": ""}, TimeoutMillis: 5_000},
		Workspaces: workspaces,
	}
}

func writeFixtureFile(t *testing.T, root, relative, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func patchReplayFixture(t *testing.T) (string, taskbundle.Request) {
	t.Helper()
	return freezeFixture(t)
}

func TestFreezeDeterministicAndRejectsMissingOrChangedInputs(t *testing.T) {
	root, request := freezeFixture(t)
	first, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatalf("first freeze: %v", err)
	}
	second, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatalf("second freeze: %v", err)
	}
	firstJSON, err := taskbundle.CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := taskbundle.CanonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("identical inputs did not produce identical manifests:\n%s\n%s", firstJSON, secondJSON)
	}
	if first.SHA256 == "" || !strings.HasPrefix(first.SHA256, "sha256:") {
		t.Fatalf("manifest is not SHA-256 sealed: %q", first.SHA256)
	}
	if err := taskbundle.Verify(root, first); err != nil {
		t.Fatalf("fresh manifest did not verify: %v", err)
	}

	writeFixtureFile(t, root, "spec.md", "# tampered contract\n", 0o644)
	if err := taskbundle.Verify(root, first); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed artifact was not rejected: %v", err)
	}

	missing := request
	missing.Artifacts = append([]taskbundle.ArtifactSpec(nil), request.Artifacts...)
	missing.Artifacts = append(missing.Artifacts, taskbundle.ArtifactSpec{ID: "absent", Kind: "tests", Path: "missing.patch"})
	if _, err := taskbundle.Freeze(root, missing); err == nil || !strings.Contains(err.Error(), "missing.patch") {
		t.Fatalf("missing declared artifact was not rejected: %v", err)
	}
}

func TestFreezeRejectsWorkspaceMutationAndManifestTampering(t *testing.T) {
	root, request := freezeFixture(t)
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "workspaces/base-old/source.py", "def answer(): return 7\n", 0o644)
	if err := taskbundle.Verify(root, manifest); err == nil || !strings.Contains(err.Error(), "workspace changed") {
		t.Fatalf("workspace mutation was not rejected: %v", err)
	}

	tampered := manifest
	tampered.Environment.Identity = "different-image"
	if err := taskbundle.Validate(tampered); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("manifest content tampering was not rejected: %v", err)
	}
}

func TestFreezeRejectsToolMutationAndDigestMismatch(t *testing.T) {
	root, request := freezeFixture(t)
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := manifest.Tool("fixture-tool")
	if !ok || tool.Path == "" || tool.SHA256 == "" || tool.VersionOutputSHA256 == "" {
		t.Fatalf("proof-critical tool identity was not frozen: %+v, %v", tool, ok)
	}
	if tool.ExpectedVersion != "fixture-tool-v1" || tool.ReportedVersion != "fixture-tool-v1\nbuild fixture-42" || tool.Version != tool.ReportedVersion {
		t.Fatalf("expected and exact reported tool versions were not preserved: %+v", tool)
	}
	detachedVersion := tool
	detachedVersion.ExpectedVersion = "unreported-version"
	if err := taskbundle.VerifyTool(context.Background(), detachedVersion); err == nil || !strings.Contains(err.Error(), "incomplete identity") {
		t.Fatalf("detached expected/reported version evidence was not rejected: %v", err)
	}
	missingVersionCommand := tool
	missingVersionCommand.VersionArgs = nil
	if err := taskbundle.VerifyTool(context.Background(), missingVersionCommand); err == nil || !strings.Contains(err.Error(), "version evidence") {
		t.Fatalf("tool identity without an exact version command was not rejected: %v", err)
	}
	writeFixtureFile(t, root, "tools/fixture-tool", "#!/bin/sh\nprintf 'fixture-tool-v2\\nbuild changed\\n'\n", 0o755)
	if err := taskbundle.Verify(root, manifest); err == nil || !strings.Contains(err.Error(), "executable changed") {
		t.Fatalf("changed executable was not rejected: %v", err)
	}

	root, request = freezeFixture(t)
	request.Environment.Tools[0].SHA256 = freezeTestDigest("not the executable")
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "executable digest mismatch") {
		t.Fatalf("caller-supplied mismatched tool digest was not rejected: %v", err)
	}

	root, request = freezeFixture(t)
	request.Environment.Tools[0].Version = "fixture-tool-v999"
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "reported version") {
		t.Fatalf("mismatched reported tool version was not rejected: %v", err)
	}
}

func TestFreezeWorkspaceTriple(t *testing.T) {
	root, request := freezeFixture(t)
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatalf("freeze workspace triple: %v", err)
	}
	wantStates := []taskbundle.WorkspaceState{
		taskbundle.BaseOldTests,
		taskbundle.BaseNewTests,
		taskbundle.SolutionNewTests,
	}
	wantPass := []bool{true, false, true}
	for i, workspace := range manifest.Workspaces {
		if workspace.State != wantStates[i] || workspace.Result.Passed != wantPass[i] {
			t.Errorf("workspace %d = (%q,%v), want (%q,%v)", i, workspace.State, workspace.Result.Passed, wantStates[i], wantPass[i])
		}
		if workspace.Result.ExitCode != 0 {
			t.Errorf("fixture command should always exit zero, got %d", workspace.Result.ExitCode)
		}
		if workspace.TreeSHA256 == "" || len(workspace.Entries) != 3 {
			t.Errorf("workspace %q was not content-addressed: %+v", workspace.State, workspace)
		}
	}
	// The command ran only in an isolated copy; verification evidence must not
	// mutate the workspace being certified.
	for _, workspace := range request.Workspaces {
		content, err := os.ReadFile(filepath.Join(root, workspace.Root, "reward.txt"))
		if err != nil || string(content) != "stale\n" {
			t.Fatalf("freeze command mutated source workspace %q: %q, %v", workspace.State, content, err)
		}
	}

	missingState := request
	missingState.Workspaces = missingState.Workspaces[:2]
	if _, err := taskbundle.Freeze(root, missingState); err == nil || !strings.Contains(err.Error(), "exactly 3") {
		t.Fatalf("incomplete triple was not rejected: %v", err)
	}

	wrongResult := request
	wrongResult.Workspaces = append([]taskbundle.WorkspaceInput(nil), request.Workspaces...)
	wrongResult.Workspaces[1].Command.PassSignal.Expected = "0\n"
	observed, err := taskbundle.Freeze(root, wrongResult)
	if err != nil || !observed.Workspaces[1].Result.Passed {
		t.Fatalf("freeze did not retain the actual base+new observation: %+v, %v", observed.Workspaces, err)
	}
}

func TestFreezeRejectsUndeclaredWorkspaceDerivation(t *testing.T) {
	root, request := freezeFixture(t)
	writeFixtureFile(t, root, "workspaces/base-new/undeclared.txt", "not bound to an artifact\n", 0o644)
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "prepared tree differs") {
		t.Fatalf("arbitrary prepared workspace was accepted: %v", err)
	}

	root, request = freezeFixture(t)
	request.Workspaces[1].Derivation.PatchArtifactIDs[0] = "solution-patch"
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "invalid for base+new-tests") {
		t.Fatalf("solution artifact was accepted as a test derivation: %v", err)
	}
}

func TestFreezeReplaysRepositoryBaseAndOrderedPatches(t *testing.T) {
	root, request := patchReplayFixture(t)
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatalf("freeze repository patch replay: %v", err)
	}
	if manifest.Repository == nil || manifest.Repository.BaseCommit != request.Repository.BaseCommit || !strings.HasPrefix(manifest.Repository.BaseTreeSHA256, "sha256:") {
		t.Fatalf("repository/base tree identity was not retained: %+v", manifest.Repository)
	}
	for index, workspace := range manifest.Workspaces {
		if workspace.PatchReplay == nil || workspace.PatchReplay.ResultTreeSHA256 != workspace.TreeSHA256 || len(workspace.PatchReplay.Applications) != index {
			t.Fatalf("workspace %q patch replay is truncated: %+v", workspace.State, workspace.PatchReplay)
		}
	}
	if err := taskbundle.VerifyCurrent(root, manifest); err != nil {
		t.Fatalf("verify current repository replay: %v", err)
	}

	root, request = patchReplayFixture(t)
	request.Repository.BaseCommit = strings.Repeat("0", 40)
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "resolve base commit") {
		t.Fatalf("wrong repository base commit was accepted: %v", err)
	}

	root, request = patchReplayFixture(t)
	request.Workspaces[2].Derivation.PatchArtifactIDs = []string{"solution-patch", "tests-patch"}
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "ordered test-patch prefix") {
		t.Fatalf("swapped test/solution patch order was accepted: %v", err)
	}

	root, request = patchReplayFixture(t)
	request.Workspaces[2].Derivation.PatchArtifactIDs = []string{"tests-patch"}
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "append declared solution patches") {
		t.Fatalf("omitted solution patch was accepted: %v", err)
	}

	root, request = patchReplayFixture(t)
	manifest, err = taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, "tests/test.patch", "stale patch bytes\n", 0o644)
	if err := taskbundle.VerifyCurrent(root, manifest); err == nil || !strings.Contains(err.Error(), "content or mode changed") {
		t.Fatalf("stale frozen test patch was accepted: %v", err)
	}

	root, request = patchReplayFixture(t)
	writeFixtureFile(t, root, "workspaces/base-new/source.py", "undeclared prepared mutation\n", 0o644)
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "prepared tree differs") {
		t.Fatalf("prepared workspace not produced by base+patches was accepted: %v", err)
	}
}

func TestFreezeCommandsDoNotInheritAmbientEnvironment(t *testing.T) {
	t.Setenv("RAY_UNDECLARED_AMBIENT", "poison")
	t.Setenv("RAY_MODE", "ambient-bypass")
	root, request := freezeFixture(t)
	for index := range request.Workspaces {
		request.Workspaces[index].Command.Text = `if [ "${RAY_UNDECLARED_AMBIENT+x}" = x ] || [ "${RAY_MODE+x}" = x ]; then printf poison > reward.txt; else cp verdict.txt reward.txt; fi`
	}
	if _, err := taskbundle.Freeze(root, request); err != nil {
		t.Fatalf("undeclared ambient environment reached a certified command: %v", err)
	}
}

func TestFreezeEmptyEnvironmentCannotResolveAmbientPATH(t *testing.T) {
	root, request := freezeFixture(t)
	for index := range request.Workspaces {
		request.Workspaces[index].Command.Environment = map[string]string{}
	}
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "reward.txt") {
		t.Fatalf("bare cp resolved through an undeclared ambient/default PATH: %v", err)
	}
}

func TestFreezePreservesToolShimInvocationPath(t *testing.T) {
	root, request := freezeFixture(t)
	shim := filepath.Join(root, "tools", "fixture-tool-shim")
	if err := os.Symlink("fixture-tool", shim); err != nil {
		t.Fatal(err)
	}
	request.Environment.Tools[0].Path = shim
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := manifest.Tool("fixture-tool")
	if !ok || tool.Path != shim {
		t.Fatalf("tool invocation shim was rewritten: %+v", tool)
	}
	if err := taskbundle.VerifyTool(context.Background(), tool); err != nil {
		t.Fatalf("frozen shim identity did not verify: %v", err)
	}
}

func TestFreezeRequiresProofCriticalCommandShell(t *testing.T) {
	root, request := freezeFixture(t)
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, workspace := range manifest.Workspaces {
		if workspace.Command.Shell != "/bin/sh" || workspace.Command.ShellToolName != "shell" {
			t.Fatalf("workspace %q shell was not bound to the frozen tool identity: %+v", workspace.State, workspace.Command)
		}
	}

	root, request = freezeFixture(t)
	request.Environment.Tools = append(request.Environment.Tools[:1], request.Environment.Tools[2:]...)
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "must match exactly one frozen tool identity") {
		t.Fatalf("unfrozen command shell was accepted: %v", err)
	}

	root, request = freezeFixture(t)
	request.Workspaces[0].Command.ShellToolName = "fixture-tool"
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "does not match frozen tool") {
		t.Fatalf("mismatched command shell tool name was accepted: %v", err)
	}
}

func TestFreezeWorkspaceTripleRejectsStaleAndBrokenVerdicts(t *testing.T) {
	root, request := freezeFixture(t)
	for i := range request.Workspaces {
		request.Workspaces[i].Command.Text = "true"
		request.Workspaces[i].Command.PassSignal.Expected = "PASS\n"
	}
	if _, err := taskbundle.Freeze(root, request); err == nil || !strings.Contains(err.Error(), "pass signal file") {
		t.Fatalf("pre-seeded verdicts certified a no-op command: %v", err)
	}

	root, request = freezeFixture(t)
	request.Workspaces[2].Command.Text = "printf '0\\n' > reward.txt"
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil || manifest.Workspaces[2].Result.Passed {
		t.Fatalf("fresh broken solution verdict was not retained for later T(C) rejection: %+v, %v", manifest.Workspaces, err)
	}
}

func TestFreezeRejectsMissingRelabeledOrOverlappingRequiredInputs(t *testing.T) {
	root, request := freezeFixture(t)
	missing := request
	missing.RequiredInputs.HiddenTestArtifactIDs = nil
	if _, err := taskbundle.Freeze(root, missing); err == nil || !strings.Contains(err.Error(), "hidden_test_artifact_ids must not be empty") {
		t.Fatalf("missing hidden verifier role was accepted: %v", err)
	}

	root, request = freezeFixture(t)
	relabeled := request
	relabeled.Artifacts = append([]taskbundle.ArtifactSpec(nil), request.Artifacts...)
	for index := range relabeled.Artifacts {
		if relabeled.Artifacts[index].ID == "hidden-test" {
			relabeled.Artifacts[index].Kind = "environment"
		}
	}
	if _, err := taskbundle.Freeze(root, relabeled); err == nil || !strings.Contains(err.Error(), "want \"tests\"") {
		t.Fatalf("configuration relabeled a hidden test artifact: %v", err)
	}

	root, request = freezeFixture(t)
	overlapping := request
	overlapping.RequiredInputs.HiddenTestArtifactIDs = []string{"public-test"}
	if _, err := taskbundle.Freeze(root, overlapping); err == nil || !strings.Contains(err.Error(), "multiple roles") {
		t.Fatalf("public/hidden role overlap was accepted: %v", err)
	}
}

func TestFreezeMixedPatchRoleRangesAreHashedAndReplayed(t *testing.T) {
	root, request := freezeFixture(t)
	testsPatch, err := os.ReadFile(filepath.Join(root, "tests", "test.patch"))
	if err != nil {
		t.Fatal(err)
	}
	solutionPatch, err := os.ReadFile(filepath.Join(root, "solution", "solution.patch"))
	if err != nil {
		t.Fatal(err)
	}
	combined := append(append([]byte(nil), testsPatch...), solutionPatch...)
	writeFixtureFile(t, root, "changes.patch", string(combined), 0o644)
	var artifacts []taskbundle.ArtifactSpec
	for _, artifact := range request.Artifacts {
		if artifact.ID != "tests-patch" && artifact.ID != "solution-patch" {
			artifacts = append(artifacts, artifact)
		}
	}
	request.Artifacts = append(artifacts, taskbundle.ArtifactSpec{ID: "mixed-patch", Kind: "patch", Path: "changes.patch"})
	request.RequiredInputs.PublicTestArtifactIDs = []string{"mixed-patch", "public-test"}
	request.RequiredInputs.SolutionArtifactIDs = []string{"mixed-patch"}
	request.RequiredInputs.RoleRanges = []taskbundle.RoleRange{
		{Role: "public-test", ArtifactID: "mixed-patch", StartByte: 0, EndByte: int64(len(testsPatch))},
		{Role: "solution", ArtifactID: "mixed-patch", StartByte: int64(len(testsPatch)), EndByte: int64(len(combined))},
	}
	request.Workspaces[1].Derivation.PatchArtifactIDs = []string{"mixed-patch"}
	request.Workspaces[2].Derivation.PatchArtifactIDs = []string{"mixed-patch"}
	manifest, err := taskbundle.Freeze(root, request)
	if err != nil {
		t.Fatalf("freeze mixed patch ranges: %v", err)
	}
	if len(manifest.RequiredInputs.RoleRanges) != 2 || manifest.RequiredInputs.RoleRanges[0].SHA256 == "" || manifest.RequiredInputs.RoleRanges[1].SHA256 == "" {
		t.Fatalf("mixed patch byte ranges were not digest-bound: %+v", manifest.RequiredInputs.RoleRanges)
	}
	if got := manifest.Workspaces[1].PatchReplay.Applications[0].RoleRanges; len(got) != 1 || got[0].Role != "public-test" {
		t.Fatalf("base+tests replay used wrong mixed-patch ranges: %+v", got)
	}
	if got := manifest.Workspaces[2].PatchReplay.Applications[0].RoleRanges; len(got) != 2 {
		t.Fatalf("solution+tests replay truncated mixed-patch ranges: %+v", got)
	}

	tampered := request
	tampered.RequiredInputs.RoleRanges = append([]taskbundle.RoleRange(nil), request.RequiredInputs.RoleRanges...)
	tampered.RequiredInputs.RoleRanges[1].StartByte--
	if _, err := taskbundle.Freeze(root, tampered); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("overlapping solution/test byte ranges were accepted: %v", err)
	}
}

func TestFreezeTimeoutTerminatesDescendantProcess(t *testing.T) {
	root, request := freezeFixture(t)
	marker := filepath.Join(t.TempDir(), "late-child-write")
	request.Workspaces[0].Command.Text = `(sleep 0.4; printf late > "$RAY_LATE_MARKER") & sleep 5`
	request.Workspaces[0].Command.Environment = map[string]string{"PATH": "/bin:/usr/bin", "RAY_LATE_MARKER": marker}
	request.Workspaces[0].Command.TimeoutMillis = 80
	if _, err := taskbundle.FreezeContext(context.Background(), root, request); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timed-out workspace command was not rejected: %v", err)
	}
	// If only the shell were killed, its background child would create this
	// marker after the timeout. Waiting beyond the child's delay proves the
	// entire process group was terminated and reaped.
	time.Sleep(650 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("timed-out descendant survived and wrote evidence: %v", err)
	}
}

func TestFreezeToolTimeoutTerminatesDescendantProcess(t *testing.T) {
	root, request := freezeFixture(t)
	marker := filepath.Join(t.TempDir(), "late-tool-child-write")
	toolPath := request.Environment.Tools[0].Path
	script := "#!/bin/sh\n(/bin/sleep 0.4; printf late > \"" + marker + "\") &\n/bin/sleep 5\n"
	if err := os.WriteFile(toolPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if _, err := taskbundle.FreezeContext(ctx, root, request); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timed-out tool version command was not rejected: %v", err)
	}
	time.Sleep(650 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("timed-out tool descendant survived and wrote evidence: %v", err)
	}
}

func TestFreezeRequestStrictJSONAndPathBoundary(t *testing.T) {
	root, request := freezeFixture(t)
	requestPath := filepath.Join(root, "freeze.json")
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(requestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := taskbundle.LoadRequest(requestPath)
	if err != nil || len(loaded.Workspaces) != 3 {
		t.Fatalf("load valid request: %+v, %v", loaded, err)
	}
	if err := os.WriteFile(requestPath, append(data[:len(data)-1], []byte(`,"typo":true}`)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := taskbundle.LoadRequest(requestPath); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown request field was not rejected: %v", err)
	}

	escape := request
	escape.Artifacts = append([]taskbundle.ArtifactSpec(nil), request.Artifacts...)
	escape.Artifacts[0].Path = "../outside"
	if _, err := taskbundle.Freeze(root, escape); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping artifact path was not rejected: %v", err)
	}

	outside := t.TempDir()
	writeFixtureFile(t, outside, "secret", "outside task boundary\n", 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "linked-outside")); err != nil {
		t.Fatal(err)
	}
	symlinkEscape := request
	symlinkEscape.Artifacts = append([]taskbundle.ArtifactSpec(nil), request.Artifacts...)
	symlinkEscape.Artifacts[0].Path = "linked-outside/secret"
	if _, err := taskbundle.Freeze(root, symlinkEscape); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("parent-symlink artifact escape was not rejected: %v", err)
	}

	overlap := request
	overlap.Workspaces = append([]taskbundle.WorkspaceInput(nil), request.Workspaces...)
	overlap.Workspaces[1].Root = filepath.Join(overlap.Workspaces[0].Root, "nested")
	if _, err := taskbundle.Freeze(root, overlap); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("overlapping workspace roots were not rejected: %v", err)
	}
}
