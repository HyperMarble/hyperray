// Missing process state must remain an error, not a library crash.
package execution

import (
	"os/exec"
	"testing"
	"time"
)

func TestMissingProcessState(t *testing.T) {
	result, err := measuredResult(&exec.Cmd{}, Request{}, &boundedOutput{}, nil, nil, time.Now())
	if err == nil || result.Status != "" {
		t.Fatalf("missing process state result = %+v, error = %v", result, err)
	}
}
