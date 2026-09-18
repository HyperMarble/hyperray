-- Make sure that a one-bit ADDIW circuit mutation fails.
-- The runner requires a counterexample from this file.
import OfficialADDISlice.AddiwProof
import Std.Tactic.BVDecide

def mutatedAddiwCircuit
    (source : BitVec 64) (immediate : BitVec 12) : BitVec 64 :=
  (addiwCircuit source immediate) ^^^ 1#64

theorem mutatedAddiwEquivalence
    (source : BitVec 64) (immediate : BitVec 12) :
    addiwSemanticValue source immediate =
      mutatedAddiwCircuit source immediate := by
  simp only [addiwSemanticValue, mutatedAddiwCircuit, addiwCircuit,
    Sail.BitVec.signExtend, Sail.BitVec.extractLsb]
  bv_decide
