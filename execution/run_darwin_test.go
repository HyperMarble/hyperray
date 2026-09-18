// Real worker tests preserve exit codes, diagnostics, and public measurements.
// A successful exit must remain distinct from correctness.
package execution_test

import (
	"context"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/execution"
)

func TestWorkerStreams(t *testing.T) {
	result, err := execution.Run(context.Background(), fixtureRequest(t, "streams"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != execution.Exited || result.ExitCode != 0 {
		t.Fatalf("unexpected execution: %+v", result)
	}
	for _, marker := range []string{"worker output\n", "worker diagnostic\n"} {
		if !strings.Contains(result.Output, marker) {
			t.Errorf("missing stream: %q", marker)
		}
	}
	if result.WorkerPeakBytes <= 0 || result.ObserverPeakBytes <= 0 || result.Elapsed <= 0 {
		t.Fatalf("missing measurement: %+v", result)
	}
	if result.CombinedPeakBytes != result.WorkerPeakBytes+result.ObserverPeakBytes {
		t.Fatal("combined memory does not equal the measured sum")
	}
}

func TestNonzeroExit(t *testing.T) {
	result, err := execution.Run(context.Background(), fixtureRequest(t, "failure"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != execution.Exited || result.ExitCode != 7 || result.Diagnostic == "" {
		t.Fatalf("worker error was hidden: %+v", result)
	}
	if result.Output != "declared worker error\n" {
		t.Fatalf("worker diagnostics changed: %q", result.Output)
	}
}

func TestExplicitEnvironment(t *testing.T) {
	t.Setenv("HYPERRAY_OBSERVER_TEST", "inherited")
	request := fixtureRequest(t, "environment")
	for _, value := range []string{"unset", "explicit"} {
		result, err := execution.Run(context.Background(), request)
		if err != nil || result.Output != value+"\n" {
			t.Fatalf("environment result = %+v, error = %v", result, err)
		}
		request.Environment = []string{"HYPERRAY_OBSERVER_TEST=explicit"}
	}
}
