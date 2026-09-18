// Command tests observe completed output and output-write errors.
// They never invoke Sail or depend on an instruction family.
package main

import (
	"bytes"
	"errors"
	"testing"
)

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("output error")
}

func TestOperateReportsCompletedCatalog(t *testing.T) {
	path := writeCatalog(t, validCatalog())
	var output bytes.Buffer
	if err := operate([]string{path}, &output); err != nil {
		t.Fatalf("operate() error = %v", err)
	}
	want := "{\"complete\":true,\"jib_definitions\":1,\"jib_origins\":12,\"instruction_kinds\":1}\n"
	if output.String() != want {
		t.Errorf("operate() output = %q, want %q", output.String(), want)
	}
}

func TestOperateReportsWriteError(t *testing.T) {
	path := writeCatalog(t, validCatalog())
	if err := operate([]string{path}, errorWriter{}); err == nil {
		t.Error("operate() error = nil")
	}
}
