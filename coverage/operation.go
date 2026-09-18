// Operation decisions expose all accepted kinds and base dispositions.
// They never treat reachability as a disposition.
package coverage

func validateOperationKind(operation Operation) error {
	switch operation.Kind {
	case OperationCompiler, OperationSynthetic, OperationEnvironment:
		return nil
	default:
		return coverageError("invalid_operation_kind", operation.ID, string(operation.Kind))
	}
}

func validateDisposition(operation Operation) error {
	switch operation.Disposition {
	case DispositionMapped:
		return nil
	case DispositionEliminated:
		if operation.Kind != OperationCompiler {
			return coverageError("invalid_disposition", operation.ID, string(operation.Kind))
		}
		return nil
	default:
		return coverageError("invalid_disposition", operation.ID, string(operation.Disposition))
	}
}
