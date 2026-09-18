// Catalog construction validates each independent input catalog in fixed order.
// It never changes its first error when slice order changes.
package coverage

import "github.com/HyperMarble/hyperray/model"

func buildCatalogs(graph model.Model, inventory CompilerInventory) (catalogs, error) {
	result := newCatalogs(graph)
	steps := []func(*catalogs, CompilerInventory) error{
		collectCore, collectArtifacts, collectEvidenceItems, collectProvenance,
		collectProofs, collectRootEntries, collectEliminations, collectBindings,
	}
	for _, step := range steps {
		if err := step(&result, inventory); err != nil {
			return catalogs{}, err
		}
	}
	if err := requireCatalogComplete(result); err != nil {
		return catalogs{}, err
	}
	return result, nil
}

func newCatalogs(graph model.Model) catalogs {
	result := catalogs{
		operations: make(map[string]Operation), artifacts: make(map[string]Artifact),
		edges:       make(map[string]ProvenanceEdge),
		rootEntries: make(map[string]RootEntry), machine: make(map[MachineBinding]struct{}),
		semantic: make(map[SemanticBinding]struct{}), eliminations: make(map[string]EliminationRecord),
		transitionOwners:  make(map[string]string),
		unsupportedProofs: make(idSet),
	}
	result.states = make(idSet, len(graph.States))
	result.transitions = make(idSet, len(graph.Transitions))
	for _, state := range graph.States {
		result.states[state.ID] = struct{}{}
	}
	for _, transition := range graph.Transitions {
		result.transitions[transition.ID] = struct{}{}
	}
	result.used = newCatalogUse()
	return result
}

func newCatalogUse() catalogUse {
	return catalogUse{
		artifacts: make(idSet), outputs: make(idSet), instructions: make(idSet),
		rules: make(idSet), provenance: make(idSet), impossible: make(idSet),
		proofs: make(idSet), operations: make(idSet),
	}
}
