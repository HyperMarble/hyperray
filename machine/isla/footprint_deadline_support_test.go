// The deadline fixture delegates normal output to the existing fake tool.
// The blocked operation retains its process identity so cancellation is observable.
package isla_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func delayedFootprintEngine(t *testing.T) (isla.FootprintEngine, string) {
	t.Helper()
	processPath := filepath.Join(t.TempDir(), "process.pid")
	body := fmt.Sprintf(`for argument in "$@"; do
case "$argument" in
--version|0100) exec %q "$@" ;;
esac
done
printf '%%s\n' "$$" > %q
while :; do
:
done`, footprintTool(t), processPath)
	engine, err := isla.NewFootprintEngine(t.Context(), temporaryTool(t, body))
	if err != nil {
		t.Fatal(err)
	}
	return engine, processPath
}

func deadlineFootprintRequest(t *testing.T, engine isla.FootprintEngine, seconds uint64) isla.FootprintRequest {
	t.Helper()
	architecture := testArtifact(t, "deadline-architecture")
	configuration := testArtifact(t, "deadline-configuration")
	release := footprintRelease(t, engine, architecture, configuration)
	instructions := []machine.Instruction{
		{Address: 2, Bytes: []byte{1, 0}},
		{Address: 4, Bytes: []byte{2, 0}},
	}
	request, err := isla.NewFootprintRequest(release, instructions, 1, seconds, 4096)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
