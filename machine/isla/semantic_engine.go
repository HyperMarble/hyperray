// Semantic engine construction identifies the exact Isla dump executable.
// The dump supplies model-generated event trees for one complete program.
package isla

import "context"

// SemanticEngine operates one identified Isla semantic-dump executable.
type SemanticEngine struct {
	identity ToolIdentity
}

// NewSemanticEngine identifies an Isla litmus-dump executable.
func NewSemanticEngine(ctx context.Context, path string) (SemanticEngine, error) {
	identity, err := identifyTool(ctx, path, "isla-litmus-dump", []string{"--version"})
	if err != nil {
		return SemanticEngine{}, err
	}
	return SemanticEngine{identity: identity}, nil
}

// Identity returns the measured semantic tool identity.
func (engine SemanticEngine) Identity() ToolIdentity {
	return engine.identity
}
