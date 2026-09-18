// Request argument tests protect the exact CAT, model, and configuration paths.
// A capability must validate the same paths that execution receives.
package isla

import (
	"reflect"
	"testing"
)

func TestRequestArgumentsBindExactCATAndConfiguration(t *testing.T) {
	request := Request{
		architecture:  Artifact{path: "/measured/arm.ir"},
		configuration: Artifact{path: "/measured/arm.toml"},
		memoryModel:   Artifact{path: "/measured/arm.cat"},
		program:       Artifact{path: "/measured/program.toml"},
		pcVisitLimit:  7, timeLimit: 11, maximumOutputSize: 13,
	}
	args := request.arguments()
	want := []string{"--herd7", "-A", "/measured/arm.ir", "-C", "/measured/arm.toml", "-m", "/measured/arm.cat", "--pc-limit", "7", "--pc-limit-mode", "error", "-s", "11", "/measured/program.toml"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("Request.arguments() = %#v, want %#v", args, want)
	}
}
