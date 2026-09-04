// Footprint engine construction records the exact external translator.
// It shares tool identification with the proof-proposal engine.
package isla

import "context"

// FootprintEngine operates one identified Isla footprint executable.
type FootprintEngine struct {
	identity ToolIdentity
}

// NewFootprintEngine identifies an Isla footprint executable.
func NewFootprintEngine(ctx context.Context, path string) (FootprintEngine, error) {
	identity, err := identifyTool(ctx, path, "isla-footprint")
	if err != nil {
		return FootprintEngine{}, err
	}
	return FootprintEngine{identity: identity}, nil
}

// Identity returns the measured tool identity.
func (engine FootprintEngine) Identity() ToolIdentity {
	return engine.identity
}
