// A miter asks whether two closed step relations produce different outputs.
// Its digest covers the exact deterministic SMT question sent to the tool.
package circuit

import (
	"crypto/sha256"
	"encoding/hex"
)

// Miter is one immutable SMT-LIB equivalence question.
type Miter struct {
	smt2        string
	digest      string
	assignments []assignmentQuery
	outputs     []outputQuery
}

// BuildMiter creates a difference query for two equal step boundaries.
func BuildMiter(reference StepRelation, candidate StepRelation) (Miter, error) {
	if err := signatureError(reference, candidate); err != nil {
		return Miter{}, err
	}
	smt2, assignments, outputs, err := writeMiter(reference, candidate)
	return finalizedMiter(smt2, assignments, outputs, err)
}

func finalizedMiter(
	smt2 string,
	assignments []assignmentQuery,
	outputs []outputQuery,
	err error,
) (Miter, error) {
	if err != nil {
		return Miter{}, err
	}
	digestBytes := sha256.Sum256([]byte(smt2))
	return Miter{
		smt2:        smt2,
		digest:      "sha256:" + hex.EncodeToString(digestBytes[:]),
		assignments: assignments,
		outputs:     outputs,
	}, nil
}

// SMT2 returns the exact solver input.
func (miter Miter) SMT2() string {
	return miter.smt2
}

// Digest returns the identity of the exact solver input.
func (miter Miter) Digest() string {
	return miter.digest
}

func miterError(miter Miter) error {
	if miter.smt2 == "" || miter.digest == "" {
		return engineError("empty_miter", "miter")
	}
	digestBytes := sha256.Sum256([]byte(miter.smt2))
	want := "sha256:" + hex.EncodeToString(digestBytes[:])
	if miter.digest != want {
		return engineError("miter_digest_mismatch", miter.digest)
	}
	return nil
}
