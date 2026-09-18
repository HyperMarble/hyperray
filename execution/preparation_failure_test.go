//go:build preparation_integration && darwin

// Compiler rejection and existing directories must remain visible errors.
// No error path may return a prepared manifest.
package execution_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparationFailures(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	directory := t.TempDir()
	request := nativePreparation(root, directory, tools)
	output, err := invokePreparation(t, preparer, request)
	if err == nil || !strings.Contains(string(output), "create output directory") {
		t.Fatalf("existing directory accepted: %v %s", err, output)
	}
	request.Directory = filepath.Join(directory, "wrong-signature")
	request.Requirement.Function = "bad_signature"
	output, err = invokePreparation(t, preparer, request)
	if err == nil || !strings.Contains(string(output), "binding returned") {
		t.Fatalf("bad signature accepted: %v %s", err, output)
	}
	log, err := os.ReadFile(filepath.Join(request.Directory, "binding.log"))
	if err != nil || !strings.Contains(string(log), "error") {
		t.Fatalf("compiler diagnostics missing: %v %s", err, log)
	}
	if _, err := os.Stat(filepath.Join(request.Directory, "prepared.json")); !os.IsNotExist(err) {
		t.Fatalf("failed build has a prepared manifest: %v", err)
	}
}

func TestPreparationMissingArtifact(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	request := nativePreparation(root, filepath.Join(t.TempDir(), "missing-artifact"), tools)
	request.Tools["clang"] = "/bin/echo"
	output, err := invokePreparation(t, preparer, request)
	if err == nil || !strings.Contains(string(output), "artifact") {
		t.Fatalf("zero exit without an artifact was accepted: %v %s", err, output)
	}
}
