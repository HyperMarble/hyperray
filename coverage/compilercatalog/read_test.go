// Consumer tests reject duplicate identities and retain every raw body position.
// They do not classify instructions or produce coverage proof.
package compilercatalog

import (
	"encoding/json"
	"testing"
)

func TestReadRetainsStatementsAndTerminators(t *testing.T) {
	input := Inventory{Version: 1, CompilationID: "build", Instances: []Instance{{
		ID: "build:function", Symbol: "function", Kind: "function", BodyStatus: "present",
		Body: &Body{Phase: "optimized", BodyDigest: "body", Positions: []Position{
			{Block: 0, Statement: intPointer(0), OperationID: "statement", Payload: json.RawMessage(`{"kind":"StorageLive"}`)},
			{Block: 0, OperationID: "terminator", Payload: json.RawMessage(`{"kind":"Return"}`)},
		}},
	}}}
	input = prepareInventory(input)
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := Read(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.Inventory.Instances[0].Body.Positions) != 2 {
		t.Fatalf("position count = %d", len(artifact.Inventory.Instances[0].Body.Positions))
	}
}

func TestReadRejectsDuplicateOperationIdentity(t *testing.T) {
	input := Inventory{Version: 1, CompilationID: "build", Instances: []Instance{{
		ID: "build:function", Symbol: "function", Kind: "function", BodyStatus: "present",
		Body: &Body{Phase: "optimized", BodyDigest: "body", Positions: []Position{
			{Block: 0, OperationID: "same", Payload: json.RawMessage(`{"kind":"StorageLive"}`)},
			{Block: 1, OperationID: "same", Payload: json.RawMessage(`{"kind":"Return"}`)},
		}},
	}}}
	input = prepareInventory(input)
	input.Instances[0].Body.Positions[1].OperationID = input.Instances[0].Body.Positions[0].OperationID
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(content)
	assertExactError(t, err, "duplicate operation identity: "+input.Instances[0].Body.Positions[0].OperationID)
}

func TestReadRetainsFrozenPositionMetadata(t *testing.T) {
	statement := 0
	input := Inventory{Version: 1, CompilationID: "build", Instances: []Instance{{
		ID: "build:function", Symbol: "function", Kind: "function", BodyStatus: "present",
		Body: &Body{Phase: "optimized", BodyDigest: "digest", Positions: []Position{{
			Block: 0, Statement: &statement, OperationID: "statement", Terminator: false,
			SourceSpan: json.RawMessage(`{"line":1}`), Payload: json.RawMessage(`{"statement_kind":"Assign"}`),
		}, {
			Block: 0, OperationID: "terminator", Terminator: true,
			SourceSpan: json.RawMessage(`{"line":2}`), Payload: json.RawMessage(`{"terminator_kind":"Return"}`),
		}}},
	}}}
	input = prepareInventory(input)
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := Read(content)
	if err != nil {
		t.Fatal(err)
	}
	position := artifact.Inventory.Instances[0].Body.Positions[1]
	if !position.Terminator || position.Statement != nil {
		t.Fatalf("terminator metadata = %#v", position)
	}
}

func TestReadRetainsOpaqueRootFacts(t *testing.T) {
	input := structuralInventory("digest")
	input.Instances[0].RootFacts = json.RawMessage(`{"version":1,"target":"riscv64gc-unknown-linux-gnu"}`)
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := Read(content)
	if err != nil {
		t.Fatal(err)
	}
	if string(artifact.Inventory.Instances[0].RootFacts) != string(input.Instances[0].RootFacts) {
		t.Fatalf("root facts changed: %s", artifact.Inventory.Instances[0].RootFacts)
	}
}

func TestReadRejectsBodyDigestMismatch(t *testing.T) {
	input := structuralInventory("wrong-digest")
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(content)
	assertExactError(t, err, "body digest mismatch for build:function")
}

func TestReadRejectsPositionOwnershipMismatch(t *testing.T) {
	input := structuralInventory("digest")
	input.Instances[0].Body.Positions[0].OperationID = "other:function:optimized:" + input.Instances[0].Body.BodyDigest + ":block:0:statement:0"
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(content)
	assertExactError(t, err, "operation ownership mismatch: other:function:optimized:"+input.Instances[0].Body.BodyDigest+":block:0:statement:0")
}

func TestReadRejectsMissingStructuralPosition(t *testing.T) {
	input := structuralInventory("digest")
	input.Instances[0].Body.Positions = input.Instances[0].Body.Positions[:1]
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(content)
	assertExactError(t, err, "body position count mismatch for build:function: expected 2, got 1")
}

func TestReadRejectsTerminatorMetadataMutation(t *testing.T) {
	input := structuralInventory("digest")
	input.Instances[0].Body.Positions[1].Terminator = false
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(content)
	assertExactError(t, err, "body position mismatch for build:function at index 1")
}

func TestReadAcceptsOmittedTerminatorFlagWithExplicitNullStatement(t *testing.T) {
	content := mutatePositionJSON(t, structuralInventory("digest"), 1, func(position map[string]any) {
		delete(position, "terminator")
	})
	artifact, err := Read(content)
	if err != nil {
		t.Fatal(err)
	}
	position := artifact.Inventory.Instances[0].Body.Positions[1]
	if position.Statement != nil {
		t.Fatalf("terminator statement = %v, want nil", position.Statement)
	}
}

func TestReadRejectsMissingStatementField(t *testing.T) {
	content := mutatePositionJSON(t, structuralInventory("digest"), 1, func(position map[string]any) {
		delete(position, "statement")
	})
	_, err := Read(content)
	assertExactError(t, err, "body position mismatch for build:function at index 1")
}

func TestReadRejectsExplicitFalseForNullStatement(t *testing.T) {
	content := mutatePositionJSON(t, structuralInventory("digest"), 1, func(position map[string]any) {
		position["terminator"] = false
	})
	_, err := Read(content)
	assertExactError(t, err, "body position mismatch for build:function at index 1")
}

func TestReadRejectsExplicitTrueForStatement(t *testing.T) {
	content := mutatePositionJSON(t, structuralInventory("digest"), 0, func(position map[string]any) {
		position["terminator"] = true
	})
	_, err := Read(content)
	assertExactError(t, err, "body position mismatch for build:function at index 0")
}

func mutatePositionJSON(t *testing.T, input Inventory, positionIndex int, mutate func(map[string]any)) []byte {
	t.Helper()
	content, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	instances := document["instances"].([]any)
	body := instances[0].(map[string]any)["body"].(map[string]any)
	positions := body["positions"].([]any)
	mutate(positions[positionIndex].(map[string]any))
	content, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func structuralInventory(digest string) Inventory {
	payload := json.RawMessage(`{"blocks":[{"index":0,"statements":[{"statement_kind":"Assign"}],"terminator":{"terminator_kind":"Return"}}]}`)
	actualDigest, _ := digestJSON(payload)
	bodyDigest := actualDigest
	if digest == "wrong-digest" {
		bodyDigest = digest
	}
	return structuralInventoryWithDigestAndOperationID(bodyDigest, "build:function:optimized:"+bodyDigest+":block:0:statement:0")
}

func structuralInventoryWithDigestAndOperationID(digest string, operationID string) Inventory {
	statement := 0
	payload := json.RawMessage(`{"blocks":[{"index":0,"statements":[{"statement_kind":"Assign"}],"terminator":{"terminator_kind":"Return"}}]}`)
	return Inventory{Version: 1, CompilationID: "build", Instances: []Instance{{
		ID: "build:function", Symbol: "function", Kind: "function", BodyStatus: "present",
		Body: &Body{Phase: "optimized", BodyDigest: digest, Payload: payload, Positions: []Position{{
			Block: 0, Statement: &statement, OperationID: operationID, Payload: json.RawMessage(`{"statement_kind":"Assign"}`),
		}, {
			Block: 0, OperationID: "build:function:optimized:" + digest + ":block:0:terminator", Terminator: true,
			Payload: json.RawMessage(`{"terminator_kind":"Return"}`),
		}}},
	}}}
}

func prepareInventory(input Inventory) Inventory {
	for instanceIndex := range input.Instances {
		instance := &input.Instances[instanceIndex]
		if instance.Body == nil {
			continue
		}
		if len(instance.Body.Payload) == 0 {
			instance.Body.Payload = json.RawMessage(`{"legacy":"payload"}`)
		}
		digest, err := digestJSON(instance.Body.Payload)
		if err != nil {
			panic(err)
		}
		instance.Body.BodyDigest = digest
		for positionIndex := range instance.Body.Positions {
			position := &instance.Body.Positions[positionIndex]
			position.OperationID = operationID(instance.ID, instance.Body.Phase, digest, *position)
		}
	}
	return input
}

func assertExactError(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || err.Error() != expected {
		t.Fatalf("error = %v, want %s", err, expected)
	}
}

func intPointer(value int) *int {
	return &value
}
