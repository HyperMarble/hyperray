package semanticir

import (
	"fmt"
	"sort"
)

// TestVectorResult is one exhaustive verifier observation. Choices is the
// complete semantic implementation vector; Accepted is the frozen verifier's
// declared pass signal for that vector.
type TestVectorResult struct {
	Choices  []BehaviorChoice `json:"choices"`
	Accepted bool             `json:"accepted"`
}

type canonicalBehaviorChoice struct {
	OperationID string             `json:"operation_id"`
	Conditions  Assignment         `json:"conditions"`
	Inputs      map[string]Literal `json:"inputs"`
	OutcomeID   string             `json:"outcome_id"`
}

type canonicalTestVector struct {
	Choices  []canonicalBehaviorChoice `json:"choices"`
	Accepted bool                      `json:"accepted"`
}

func canonicalizeTestVector(result TestVectorResult, vectorIndex int) (canonicalTestVector, error) {
	choices := make([]canonicalBehaviorChoice, 0, len(result.Choices))
	seen := map[string]struct{}{}
	for _, choice := range result.Choices {
		key := BehaviorRefKey(choice.Behavior)
		if choice.Behavior.OperationID == "" || choice.Behavior.Inputs == nil || choice.OutcomeID == "" {
			return canonicalTestVector{}, fmt.Errorf("vector %d has an empty behavior/outcome", vectorIndex)
		}
		if _, exists := seen[key]; exists {
			return canonicalTestVector{}, fmt.Errorf("vector %d repeats behavior %s", vectorIndex, key)
		}
		seen[key] = struct{}{}
		choices = append(choices, canonicalBehaviorChoice{OperationID: choice.Behavior.OperationID, Conditions: choice.Behavior.Conditions, Inputs: choice.Behavior.Inputs, OutcomeID: choice.OutcomeID})
	}
	sort.Slice(choices, func(i, j int) bool {
		left := BehaviorRefKey(BehaviorRef{OperationID: choices[i].OperationID, Conditions: choices[i].Conditions, Inputs: choices[i].Inputs})
		right := BehaviorRefKey(BehaviorRef{OperationID: choices[j].OperationID, Conditions: choices[j].Conditions, Inputs: choices[j].Inputs})
		return left < right
	})
	return canonicalTestVector{Choices: choices, Accepted: result.Accepted}, nil
}

// TestVectorDigests returns the exact digest convention used by
// TestSuiteModel. vectorEvidence is Digest of every canonical vector plus its
// Accepted bit. acceptedVectors is Digest of the canonical vectors whose bit
// is true. Choices and vectors are sorted by canonical semantic content, so
// execution scheduling cannot perturb the evidence hash.
func TestVectorDigests(results []TestVectorResult) (vectorEvidence, acceptedVectors string, err error) {
	vectors := make([]canonicalTestVector, 0, len(results))
	for vectorIndex, result := range results {
		vector, canonicalErr := canonicalizeTestVector(result, vectorIndex)
		if canonicalErr != nil {
			return "", "", canonicalErr
		}
		vectors = append(vectors, vector)
	}
	sort.Slice(vectors, func(i, j int) bool {
		left, _ := CanonicalJSON(vectors[i].Choices)
		right, _ := CanonicalJSON(vectors[j].Choices)
		return string(left) < string(right)
	})
	vectorEvidence, err = Digest(vectors)
	if err != nil {
		return "", "", err
	}
	accepted := make([][]canonicalBehaviorChoice, 0)
	for _, vector := range vectors {
		if vector.Accepted {
			accepted = append(accepted, vector.Choices)
		}
	}
	acceptedVectors, err = Digest(accepted)
	if err != nil {
		return "", "", err
	}
	return vectorEvidence, acceptedVectors, nil
}
