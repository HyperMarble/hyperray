// Witness scalars accept numeric tool formats and reject unresolved names.
// Malformed or duplicated assignments cannot identify a concrete state.
package isla

import "testing"

func TestConcreteWitnessStates(t *testing.T) {
	for _, state := range []string{"x=1;", "0:x10=#x0040;", "x=#b01;", "x=0x40;y=true;", "x=-2;y=false;"} {
		if !concreteState(state) {
			t.Errorf("rejected concrete state %q", state)
		}
	}
	for _, state := range []string{"", "x=1", "x=v2;", "???;", "=2;", "x=1;x=2;", "x=;"} {
		if concreteState(state) {
			t.Errorf("accepted nonconcrete state %q", state)
		}
	}
}
