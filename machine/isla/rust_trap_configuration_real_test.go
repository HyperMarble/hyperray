//go:build isla_integration

// The trap notification has the unit result declared by matching upstream source.
// This fixture configuration must not replace architectural trap handling.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"
)

func useTrapNotificationConfiguration(t *testing.T) {
	t.Helper()
	content, err := os.ReadFile(requiredPath(t, "HYPERRAY_ISLA_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\ntrap_callback = \"()\"\n")...)
	path := filepath.Join(t.TempDir(), "trap-notification.toml")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPERRAY_ISLA_CONFIG", path)
}
