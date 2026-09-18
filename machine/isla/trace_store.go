// Semantics kept on disk, named by what produced them. The same encoding
// under the same model has the same semantics, in this run or a later one.
package isla

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// TraceStore holds instruction traces between runs.
//
// An entry is named by the encoding and the architecture that produced it, so
// a different model cannot return a trace it did not produce.
type TraceStore struct {
	root string
}

// NewTraceStore keeps traces under root. An empty root disables the store.
func NewTraceStore(root string) *TraceStore {
	return &TraceStore{root: root}
}

func (store *TraceStore) path(encoding string, architecture string) string {
	sum := sha256.Sum256([]byte(architecture + ":" + encoding))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(store.root, name[:2], name[2:]+".json.gz")
}

// Lookup returns a stored trace, and whether one was found.
func (store *TraceStore) Lookup(encoding string, architecture string) (InstructionTrace, bool) {
	if store.root == "" {
		return InstructionTrace{}, false
	}
	packed, err := os.ReadFile(store.path(encoding, architecture))
	if err != nil {
		return InstructionTrace{}, false
	}
	content, err := unpack(packed)
	if err != nil {
		return InstructionTrace{}, false
	}
	var trace InstructionTrace
	if err := json.Unmarshal(content, &trace); err != nil {
		return InstructionTrace{}, false
	}
	return trace, true
}

// Keep stores one trace. A failure to store is reported, never hidden.
func (store *TraceStore) Keep(encoding string, architecture string, trace InstructionTrace) error {
	if store.root == "" {
		return nil
	}
	content, err := json.Marshal(trace)
	if err != nil {
		return err
	}
	packed, err := pack(content)
	if err != nil {
		return err
	}
	target := store.path(encoding, architecture)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, packed, 0o644)
}
