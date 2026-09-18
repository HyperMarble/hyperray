-- Make sure that a one-bit register circuit mutation fails.
-- The runner requires a counterexample from this file.
import OfficialADDISlice.RTypeProof
import Std.Tactic.BVDecide

def mutatedRTypeCircuit
    (source1 source2 : BitVec 64) (operation : rop) : BitVec 64 :=
  (rTypeCircuit source1 source2 operation) ^^^ 1#64

theorem mutatedRTypeAddEquivalence (source1 source2 : BitVec 64) :
    rTypeSemanticValue source1 source2 .ADD =
      mutatedRTypeCircuit source1 source2 .ADD := by
  simp only [rTypeSemanticValue, mutatedRTypeCircuit, rTypeCircuit]
  bv_decide
