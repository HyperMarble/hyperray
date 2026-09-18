//go:build isla_integration

// This test drives a real isla-client and measures what a live session costs.
// It is skipped unless the tool, model and configuration are all supplied.
package isla_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func clientInputs(t *testing.T) (string, string, string) {
	t.Helper()
	tool := os.Getenv("HYPERRAY_ARM64_ISLA_CLIENT")
	architecture := os.Getenv("HYPERRAY_ARM64_SAIL_IR")
	configuration := os.Getenv("HYPERRAY_ARM64_ISLA_CONFIG")
	if tool == "" || architecture == "" || configuration == "" {
		t.Skip("isla-client, model or configuration not supplied")
	}
	return tool, architecture, configuration
}

func TestClientSessionAnswersManyInstructions(t *testing.T) {
	tool, architecture, configuration := clientInputs(t)
	started := time.Now()
	session, err := isla.StartClientSession(context.Background(), tool, architecture, configuration, 1)
	if err != nil {
		t.Fatalf("StartClientSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()
	setup := time.Since(started)

	version, err := session.Version()
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version == "" {
		t.Fatal("Version() returned nothing")
	}

	for _, encoding := range []string{"d503201f", "8b010000", "d503201f"} {
		instruction := time.Now()
		traces, err := session.Execute(encoding)
		if err != nil {
			t.Fatalf("Execute(%s) error = %v", encoding, err)
		}
		if len(traces) == 0 {
			t.Fatalf("Execute(%s) returned no traces", encoding)
		}
		t.Logf("%s: %d traces in %v (setup was %v)", encoding, len(traces), time.Since(instruction), setup)
	}
}

func TestClientSessionRejectsAnUnknownInstruction(t *testing.T) {
	tool, architecture, configuration := clientInputs(t)
	session, err := isla.StartClientSession(context.Background(), tool, architecture, configuration, 1)
	if err != nil {
		t.Fatalf("StartClientSession() error = %v", err)
	}
	defer func() { _ = session.Close() }()
	if _, err := session.Execute("zzzzzzzz"); err == nil {
		t.Fatal("Execute() accepted an encoding that is not hexadecimal")
	}
}
