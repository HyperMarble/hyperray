// Catalog storage separates declarations from certificate evidence.
// It never derives a declaration from a certificate row.
package coverage

type catalogs struct {
	functions         idSet
	roots             idSet
	operations        map[string]Operation
	states            idSet
	transitions       idSet
	artifacts         map[string]Artifact
	artifactIDs       idSet
	outputs           idSet
	instructions      idSet
	rules             idSet
	provenance        idSet
	edges             map[string]ProvenanceEdge
	impossible        idSet
	proofs            idSet
	rootEntries       map[string]RootEntry
	machine           map[MachineBinding]struct{}
	semantic          map[SemanticBinding]struct{}
	eliminations      map[string]EliminationRecord
	transitionOwners  map[string]string
	unsupportedProofs idSet
	used              catalogUse
}

type catalogUse struct {
	artifacts    idSet
	outputs      idSet
	instructions idSet
	rules        idSet
	provenance   idSet
	impossible   idSet
	proofs       idSet
	operations   idSet
}
