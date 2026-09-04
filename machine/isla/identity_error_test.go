// Identity error tests operate temporary executables with controlled behavior.
// They require every discovery failure to have a stable error code.
package isla_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestEngineIdentityErrors(t *testing.T) {
	engine, err := isla.NewEngine(nil, "")
	assertEngineError(t, engine, err, isla.InvalidInput)
	engine, err = isla.NewEngine(t.Context(), filepath.Join(t.TempDir(), "missing"))
	assertEngineError(t, engine, err, isla.ToolNotFound)
	engine, err = isla.NewEngine(t.Context(), temporaryTool(t, "exit 4"))
	assertEngineError(t, engine, err, isla.ToolIdentityFail)
	engine, err = isla.NewEngine(t.Context(), temporaryTool(t, "exit 0"))
	assertEngineError(t, engine, err, isla.ToolIdentityFail)
}

func TestEngineIdentityFileDisappears(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1\nrm \"$0\"")
	engine, err := isla.NewEngine(context.Background(), path)
	assertEngineError(t, engine, err, isla.ToolIdentityFail)
}

func TestEngineFindsDefaultTool(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1")
	defaultPath := filepath.Join(filepath.Dir(path), "isla-axiomatic")
	if err := os.Rename(path, defaultPath); err != nil {
		t.Fatalf("os.Rename() error = %v", err)
	}
	t.Setenv("PATH", filepath.Dir(defaultPath))
	engine, err := isla.NewEngine(t.Context(), "")
	if err != nil {
		t.Errorf("NewEngine(default) error = %v", err)
	}
	if engine.Identity().Path != defaultPath {
		t.Errorf("Identity().Path = %q, want %q", engine.Identity().Path, defaultPath)
	}
}

func TestEngineRejectsRelativeToolResult(t *testing.T) {
	path := temporaryTool(t, "printf '%s\\n' v1")
	t.Chdir(filepath.Dir(path))
	engine, err := isla.NewEngine(t.Context(), "./"+filepath.Base(path))
	assertEngineError(t, engine, err, isla.ToolNotFound)
}

func assertEngineError(t *testing.T, engine isla.Engine, err error, code isla.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("engine = %#v, error = nil", engine)
	}
	assertErrorCode(t, err, code)
}

func temporaryTool(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool")
	source := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(source), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
