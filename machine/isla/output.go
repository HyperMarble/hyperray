// Command output keeps stdout, diagnostics, and elapsed time together.
// It must preserve tool text for exact failure reports and evidence.
package isla

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type commandOutput struct {
	stdout      string
	diagnostics string
	elapsed     time.Duration
}

func rawOutputDigest(output commandOutput) string {
	content := append([]byte(output.stdout), 0)
	content = append(content, output.diagnostics...)
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
