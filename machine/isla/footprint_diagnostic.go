// Diagnostic dispositions make every accepted external warning explicit.
package isla

// DiagnosticKind identifies one external diagnostic class.
type DiagnosticKind string

const (
	// UnavailablePrimitive identifies a primitive without an Isla implementation.
	UnavailablePrimitive DiagnosticKind = "unavailable_primitive"
)

// DiagnosticDispositionKind identifies the evidence-backed treatment.
type DiagnosticDispositionKind string

const (
	// NotCalledInCompletedExecution records successful execution without the primitive.
	NotCalledInCompletedExecution DiagnosticDispositionKind = "not_called_in_completed_execution"
)

// DiagnosticDisposition binds one diagnostic to its trace evidence.
type DiagnosticDisposition struct {
	Message        string                    `json:"message"`
	Kind           DiagnosticKind            `json:"kind"`
	Disposition    DiagnosticDispositionKind `json:"disposition"`
	EvidenceDigest string                    `json:"evidence_sha256"`
}
