// Unsupported proof checks run after all structural reconciliation succeeds.
// They never let an unvalidated theorem produce a complete report.
package coverage

func requireSupportedProofs(catalog catalogs) error {
	references := sortedIDs(catalog.unsupportedProofs)
	if len(references) == 0 {
		return nil
	}
	return coverageError("unsupported_proof", references...)
}
