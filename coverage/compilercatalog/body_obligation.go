// Body obligations identify opaque payloads that cannot establish structure.
// They keep missing producer evidence visible in the public report.
package compilercatalog

import "encoding/json"

func hasOpaqueBodyPayload(inventory Inventory) bool {
	for _, instance := range inventory.Instances {
		if instance.Body != nil && !hasStructuralPayload(instance.Body.Payload) {
			return true
		}
	}
	return false
}

func hasOpaqueRootFacts(inventory Inventory) bool {
	for _, instance := range inventory.Instances {
		if len(instance.RootFacts) != 0 && string(instance.RootFacts) != "null" {
			return true
		}
	}
	return false
}

func hasStructuralPayload(payload json.RawMessage) bool {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	_, exists := value["blocks"]
	return exists
}
