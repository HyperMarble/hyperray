-- Make sure that a one-bit shift-immediate circuit mutation fails.
-- The runner requires a counterexample from this file.
import OfficialADDISlice.ShiftIProof
import Std.Tactic.BVDecide

def mutatedShiftImmediateCircuit
    (source : BitVec 64) (shift : BitVec 6) (operation : sop) : BitVec 64 :=
  (shiftImmediateCircuit source shift operation) ^^^ 1#64

theorem mutatedShiftLeftImmediateEquivalence
    (source : BitVec 64) (shift : BitVec 6) :
    shiftImmediateSemanticValue source shift .SLLI =
      mutatedShiftImmediateCircuit source shift .SLLI := by
  simp only [shiftImmediateSemanticValue, mutatedShiftImmediateCircuit,
    shiftImmediateCircuit, Sail.shift_bits_left]
  bv_decide
