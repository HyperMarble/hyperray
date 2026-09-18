// Proposal-engine identity binds each result to one executable and version.
// Discovery never permits a missing or unidentified executable.
package circuit

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ToolIdentity records the exact proposal-tool executable and version text.
type ToolIdentity struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// DifferenceEngine is one identified proposal engine.
type DifferenceEngine struct {
	identity ToolIdentity
}

// NewDifferenceEngine identifies one supported engine executable.
func NewDifferenceEngine(path string) (DifferenceEngine, error) {
	resolved, err := resolveToolPath(path)
	if err != nil {
		return DifferenceEngine{}, err
	}
	output, commandError := exec.Command(resolved, "--version").CombinedOutput()
	if commandError != nil {
		return DifferenceEngine{}, engineError("tool_identity_error", resolved, commandError.Error())
	}
	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, "Z3 version ") {
		return DifferenceEngine{}, engineError("unsupported_tool_identity", resolved, version)
	}
	digest, digestError := executableDigest(resolved)
	return identifiedTool(resolved, version, digest, digestError)
}

func identifiedTool(path string, version string, digest string, err error) (DifferenceEngine, error) {
	if err != nil {
		return DifferenceEngine{}, err
	}
	identity := ToolIdentity{Path: path, Version: version, Digest: digest}
	return DifferenceEngine{identity: identity}, nil
}

// Identity returns the tool identity that each proposal records.
func (tool DifferenceEngine) Identity() ToolIdentity {
	return tool.identity
}

func resolveToolPath(path string) (string, error) {
	name := path
	if name == "" {
		name = "z3"
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", engineError("tool_not_found", name, err.Error())
	}
	if !filepath.IsAbs(resolved) {
		return "", engineError("tool_path_not_absolute", resolved)
	}
	return filepath.Clean(resolved), nil
}
