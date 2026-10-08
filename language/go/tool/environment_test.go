// Purpose: a setting whose name or value looks like a secret is recorded as
// set with its value hidden; a plain value is kept.
// Never:   writes a credential into the record.
package tool

import "testing"

func TestAHiddenSettingIsRecordedWithoutItsValue(t *testing.T) {
	if shown("GOAUTH", "netrc") != nil || shown("GOPROXY", "https://me:pw@proxy.example") != nil {
		t.Fatal("a secret value was shown")
	}
	if value := shown("GOFLAGS", "-trimpath"); value == nil || *value != "-trimpath" {
		t.Fatal("a plain value was hidden")
	}
}
