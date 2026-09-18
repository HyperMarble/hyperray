// Body validation retains compiler positions and checks their owned identity.
// It never interprets opaque compiler metadata as semantic coverage.
package compilercatalog

import "fmt"

func validateBody(instance Instance) error {
	body := instance.Body
	if body.Phase == "" {
		return fmt.Errorf("empty body phase for %s", instance.ID)
	}
	if body.BodyDigest == "" {
		return fmt.Errorf("empty body digest for %s", instance.ID)
	}
	digest, err := digestJSON(body.Payload)
	if err != nil {
		return fmt.Errorf("decode body payload for %s: %w", instance.ID, err)
	}
	if digest != body.BodyDigest {
		return fmt.Errorf("body digest mismatch for %s", instance.ID)
	}
	if err := validateOperationIDs(instance); err != nil {
		return err
	}
	return validateStructuralPositions(instance)
}

func validateOperationIDs(instance Instance) error {
	for _, position := range instance.Body.Positions {
		expected := operationID(instance.ID, instance.Body.Phase, instance.Body.BodyDigest, position)
		if position.OperationID != expected {
			return fmt.Errorf("operation ownership mismatch: %s", position.OperationID)
		}
	}
	return nil
}

func operationID(instanceID string, phase string, bodyDigest string, position Position) string {
	if position.Statement == nil {
		return fmt.Sprintf("%s:%s:%s:block:%d:terminator", instanceID, phase, bodyDigest, position.Block)
	}
	return fmt.Sprintf("%s:%s:%s:block:%d:statement:%d", instanceID, phase, bodyDigest, position.Block, *position.Statement)
}
