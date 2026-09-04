// Footprint identity tests exercise the public constructor with a real process.
// They require the same measured identity contract as the proposal engine.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicFootprintEngineIdentity(t *testing.T) {
	path := fixtureTool(t)
	engine, err := isla.NewFootprintEngine(t.Context(), path)
	if err != nil {
		t.Fatalf("NewFootprintEngine() error = %v", err)
	}
	identity := engine.Identity()
	if identity.Path != path || identity.Version != "v0.2.0/test" {
		t.Errorf("Identity() = %#v", identity)
	}
}

func TestFootprintEngineRejectsNilContext(t *testing.T) {
	engine, err := isla.NewFootprintEngine(nil, "")
	if err == nil {
		t.Fatalf("engine = %#v, error = nil", engine)
	}
	assertErrorCode(t, err, isla.InvalidInput)
}
