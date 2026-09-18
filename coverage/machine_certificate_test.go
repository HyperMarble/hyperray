// Machine certificate tests reject duplicate and omitted independent rows.
// They never infer a claim from the transition catalog.
package coverage_test

import "testing"

func TestMachineCertificateDuplicate(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Machine = append(request.Certificate.Machine,
		request.Certificate.Machine[0])
	requireCoverageError(t, request, "duplicate_evidence")
}

func TestMachineCertificateMissing(t *testing.T) {
	request := completeRequest(t)
	request.Certificate.Machine = nil
	requireCoverageError(t, request, "missing_machine_mapping")
}

func TestMachineCertificateMismatch(t *testing.T) {
	request := completeRequest(t)
	addSharedCompilerOperation(&request)
	edge := request.Inventory.ProvenanceEdges[4]
	edge.ID = "edge:output-instruction-copy"
	request.Inventory.ProvenanceEdges = append(request.Inventory.ProvenanceEdges, edge)
	request.Inventory.MachineBindings[1].CompilerOutputToInstructionEdgeID = edge.ID
	request.Certificate.Machine[0].CompilerOutputToInstructionEdgeID = edge.ID
	requireCoverageError(t, request, "machine_mapping_mismatch")
}
