// The result joins independently validated static and query inventories.
// Neither a partial static report nor an unknown executed instruction is accepted.
package isla

func joinedExecutableResult(program Program, footprints FootprintReport, result VerificationResult) (ExecutableResult, error) {
	coverage, err := ValidateFootprintInventory(program.instructions, footprints)
	if err != nil {
		return ExecutableResult{}, err
	}
	if err := matchProgramSemantics(program, result.Semantics); err != nil {
		return ExecutableResult{}, err
	}
	return ExecutableResult{
		Program: program.Evidence(), Footprints: footprints, StaticCoverage: coverage,
		Execution: programExecutionInventory(program, result.Semantics), Verification: result,
	}, nil
}
