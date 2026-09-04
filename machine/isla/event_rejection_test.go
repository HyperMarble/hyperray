// Event-catalog rejection tests keep invalid traces out of translation.
package isla_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestTraceEventCatalogRejectsInvalidInput(t *testing.T) {
	_, report := coverageFixture(t)
	catalog, err := isla.InventoryTraceEvents(nil, report)
	if err == nil || catalog.Complete {
		t.Errorf("catalog = %#v, error = %v", catalog, err)
	}
}

func TestTraceEventCatalogRejectsMalformedTrace(t *testing.T) {
	instructions, report := coverageFixture(t)
	report.Instructions[0].TraceOutput = "(trace)"
	content := append([]byte(report.Instructions[0].TraceOutput), 0)
	content = append(content, report.Instructions[0].Diagnostics...)
	digest := sha256.Sum256(content)
	report.Instructions[0].OutputDigest = hex.EncodeToString(digest[:])
	catalog, err := isla.InventoryTraceEvents(instructions, report)
	if err == nil || catalog.Complete {
		t.Errorf("catalog = %#v, error = %v", catalog, err)
	}
	assertErrorCode(t, err, isla.ProtocolError)
}
