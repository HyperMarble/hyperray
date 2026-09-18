// ARM execution manifests are strict measured release records.
// Duplicate keys must not let stale identity values bypass validation.
package isla

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestARM64ExecutionManifestRejectsDuplicateJSONKeys(t *testing.T) {
	content, err := json.Marshal(validARM64ExecutionManifest())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	duplicate := strings.Replace(string(content), "{", `{"capability_id":"stale",`, 1)
	path := t.TempDir() + "/manifest.json"
	if err := os.WriteFile(path, []byte(duplicate), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if _, err := readARM64ExecutionManifest(path); err == nil {
		t.Fatal("readARM64ExecutionManifest() accepted duplicate identity key")
	}
}
