// Engine construction records the exact Isla binary before proof work.
// It must not identify a tool from its file name.
package isla

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToolIdentity records the executable used for one proof proposal.
type ToolIdentity struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
}

// Engine operates one identified Isla executable.
type Engine struct {
	identity ToolIdentity
}

// NewEngine identifies an Isla executable by its output and content.
func NewEngine(ctx context.Context, path string) (Engine, error) {
	if ctx == nil {
		return Engine{}, engineError(InvalidInput, "context", "nil")
	}
	resolved, err := resolveTool(path)
	if err != nil {
		return Engine{}, err
	}
	output, commandError := exec.CommandContext(ctx, resolved, "--version").CombinedOutput()
	if commandError != nil {
		return Engine{}, engineError(ToolIdentityFail, resolved, commandError.Error())
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return Engine{}, engineError(ToolIdentityFail, resolved, "empty version")
	}
	digest, err := fileDigest(resolved)
	if err != nil {
		return Engine{}, err
	}
	return Engine{identity: ToolIdentity{Path: resolved, Version: version, Digest: digest}}, nil
}

// Identity returns the measured tool identity.
func (engine Engine) Identity() ToolIdentity {
	return engine.identity
}

func resolveTool(path string) (string, error) {
	name := path
	if name == "" {
		name = "isla-axiomatic"
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", engineError(ToolNotFound, name, err.Error())
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", engineError(ToolNotFound, resolved, err.Error())
	}
	return filepath.Clean(absolute), nil
}
