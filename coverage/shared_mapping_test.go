// Shared-mapping setup gives two operations one exact transition semantics.
// It never changes the instruction, rule, or transition identity.
package coverage_test

import "github.com/HyperMarble/hyperray/coverage"

func addSharedCompilerOperation(request *testRequest) {
	request.Inventory.Operations = append(request.Inventory.Operations, coverage.Operation{
		ID: "compiled-alias", FunctionID: "main", Kind: coverage.OperationCompiler,
		Location: "main.go:12", Disposition: coverage.DispositionMapped,
	})
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges,
		coverage.ProvenanceEdge{
			ID:       "edge:compiled-alias-output",
			From:     coverage.ProvenanceNode{Kind: coverage.NodeOperation, ID: "compiled-alias"},
			To:       coverage.ProvenanceNode{Kind: coverage.NodeCompilerOutput, ID: "ssa:compiled"},
			Artifact: request.Inventory.CompilerOutputs[0].Artifact,
		})
	binding := request.Inventory.MachineBindings[0]
	binding.OperationID = "compiled-alias"
	binding.OperationToCompilerOutputEdgeID = "edge:compiled-alias-output"
	request.Inventory.MachineBindings = append(request.Inventory.MachineBindings, binding)
	mapping := request.Certificate.Machine[0]
	mapping.OperationID = "compiled-alias"
	mapping.OperationToCompilerOutputEdgeID = "edge:compiled-alias-output"
	request.Certificate.Machine = append(request.Certificate.Machine, mapping)
}
