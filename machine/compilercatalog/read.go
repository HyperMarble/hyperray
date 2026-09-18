// Read decodes one producer inventory and checks its independent identities.
// It retains all raw positions and never maps them to machine instructions.
package compilercatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func Read(content []byte) (Artifact, error) {
	var inventory Inventory
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inventory); err != nil {
		return Artifact{}, fmt.Errorf("decode inventory: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Artifact{}, fmt.Errorf("inventory has trailing data")
	}
	if inventory.Version != 1 {
		return Artifact{}, catalogError("unsupported inventory version: %d", inventory.Version)
	}
	if inventory.CompilationID == "" {
		return Artifact{}, catalogError("empty compilation identity")
	}
	if err := validateInstances(inventory.CompilationID, inventory.Instances); err != nil {
		return Artifact{}, err
	}
	digest := sha256.Sum256(content)
	return Artifact{Inventory: inventory, Content: append([]byte(nil), content...), SHA256: hex.EncodeToString(digest[:])}, nil
}
func validateInstances(compilationID string, instances []Instance) error {
	seenInstances := make(map[string]struct{}, len(instances))
	seenOperations := make(map[string]struct{})
	for _, instance := range instances {
		if instance.ID == "" {
			return catalogError("empty instance identity")
		}
		if _, exists := seenInstances[instance.ID]; exists {
			return catalogError("duplicate instance identity: %s", instance.ID)
		}
		seenInstances[instance.ID] = struct{}{}
		if !strings.HasPrefix(instance.ID, compilationID+":") {
			return catalogError("cross-build instance identity: %s", instance.ID)
		}
		if instance.BodyStatus == "present" && instance.Body == nil {
			return catalogError("present body missing: %s", instance.ID)
		}
		if instance.BodyStatus == "absent" && instance.Body != nil {
			return catalogError("absent body has payload: %s", instance.ID)
		}
		if instance.Body == nil {
			continue
		}
		for _, position := range instance.Body.Positions {
			if position.OperationID == "" {
				return catalogError("empty operation identity")
			}
			if _, exists := seenOperations[position.OperationID]; exists {
				return catalogError("duplicate operation identity: %s", position.OperationID)
			}
			seenOperations[position.OperationID] = struct{}{}
		}
		if err := validateBody(instance); err != nil {
			return err
		}
	}
	return nil
}
