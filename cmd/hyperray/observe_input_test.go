// Observation requests use the existing strict JSON boundary.
// Unknown fields and trailing documents must not reach the worker.
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestObserveInvalidInput(t *testing.T) {
	for _, content := range []string{`{"unexpected":1}`, `{} {}`, `{`, `{}`} {
		command := newRootCmd()
		output := new(bytes.Buffer)
		command.SetOut(output)
		command.SetArgs([]string{"observe", writeFixture(t, "request.json", []byte(content))})
		if err := command.Execute(); err == nil {
			t.Errorf("invalid input accepted: %q", content)
		}
		if output.Len() != 0 {
			t.Errorf("invalid input produced a result: %q", output.String())
		}
	}
}

func TestObserveHelp(t *testing.T) {
	command := newRootCmd()
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetArgs([]string{"observe", "--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "does not prove correctness") {
		t.Fatal("missing diagnostic-only boundary in help")
	}
}
