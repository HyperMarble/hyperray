// Check applies the complete semantic-coverage contract to one validated model.
// It never returns a complete report after an evidence error.
package coverage

import "github.com/HyperMarble/hyperray/model"

// Check validates exact explicit-graph coverage and returns an opaque snapshot.
func Check(graph model.Model, inventory CompilerInventory, certificate Certificate) (ValidatedCoverage, error) {
	orderedGraph := canonicalModel(graph)
	if err := model.Validate(orderedGraph); err != nil {
		return ValidatedCoverage{}, &Error{
			Code:       "invalid_model",
			References: []string{err.Error()},
			cause:      err,
		}
	}
	catalog, err := buildCatalogs(orderedGraph, inventory)
	if err != nil {
		return ValidatedCoverage{}, err
	}
	if err := checkRootMappings(catalog, certificate.Roots); err != nil {
		return ValidatedCoverage{}, err
	}
	if err := checkMachineMappings(catalog, certificate.Machine); err != nil {
		return ValidatedCoverage{}, err
	}
	if err := checkSemanticMappings(catalog, certificate.Synthetic, OperationSynthetic); err != nil {
		return ValidatedCoverage{}, err
	}
	if err := checkSemanticMappings(catalog, certificate.Environment, OperationEnvironment); err != nil {
		return ValidatedCoverage{}, err
	}
	if err := checkEliminations(catalog, certificate.Eliminated); err != nil {
		return ValidatedCoverage{}, err
	}
	if err := requireSupportedProofs(catalog); err != nil {
		return ValidatedCoverage{}, err
	}
	return ValidatedCoverage{
		report: coverageReport(catalog), graph: orderedGraph,
		roots: catalog.rootEntries, validated: true,
	}, nil
}
