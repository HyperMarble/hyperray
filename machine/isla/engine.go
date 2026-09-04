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
	identity, err := identifyTool(ctx, path, "isla-axiomatic")
	if err != nil {
		return Engine{}, err
	}
	return Engine{identity: identity}, nil
}

func identifyTool(ctx context.Context, path string, defaultName string) (ToolIdentity, error) {
	if ctx == nil {
		return ToolIdentity{}, engineError(InvalidInput, "context", "nil")
	}
	resolved, err := resolveTool(path, defaultName)
	if err != nil {
		return ToolIdentity{}, err
	}
	output, commandError := exec.CommandContext(ctx, resolved, "--version").CombinedOutput()
	if commandError != nil {
		return ToolIdentity{}, engineError(ToolIdentityFail, resolved, commandError.Error())
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return ToolIdentity{}, engineError(ToolIdentityFail, resolved, "empty version")
	}
	digest, err := fileDigest(resolved)
	if err != nil {
		return ToolIdentity{}, engineError(ToolIdentityFail, resolved, err.Error())
	}
	return ToolIdentity{Path: resolved, Version: version, Digest: digest}, nil
}

// Identity returns the measured tool identity.
func (engine Engine) Identity() ToolIdentity {
	return engine.identity
}

func resolveTool(path string, defaultName string) (string, error) {
	name := path
	if name == "" {
		name = defaultName
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", engineError(ToolNotFound, name, err.Error())
	}
	if !filepath.IsAbs(resolved) {
		return "", engineError(ToolNotFound, resolved, "tool path is not absolute")
	}
	return filepath.Clean(resolved), nil
}
