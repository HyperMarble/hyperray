-- Prove that the official shift-immediate family agrees with bit circuits.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

theorem shiftImmediateFieldIdentity (shift : BitVec 6) :
    shift.extractLsb 5 0 = shift := by
  bv_decide

def shiftImmediateSemanticValue
    (source : BitVec 64) (shift : BitVec 6) (operation : sop) : BitVec 64 :=
  match operation with
  | .SLLI => Sail.shift_bits_left source shift
  | .SRLI => Sail.shift_bits_right source shift
  | .SRAI => shift_bits_right_arith source shift

def shiftImmediateStateAction
    (shift : BitVec 6) (sourceRegister destination : regidx)
    (operation : sop) : SailM ExecutionResult := do
  let source ← rX_bits sourceRegister
  wX_bits destination (shiftImmediateSemanticValue source shift operation)
  pure RETIRE_SUCCESS

theorem officialExecuteShiftImmediateBridge
    (shift : BitVec 6) (sourceRegister destination : regidx)
    (operation : sop) :
    execute_SHIFTIOP shift sourceRegister destination operation =
      shiftImmediateStateAction shift sourceRegister destination operation := by
  cases operation <;>
    simp [execute_SHIFTIOP, shiftImmediateStateAction,
      shiftImmediateSemanticValue, OfficialADDISlice.Functions.log2_xlen,
      Sail.BitVec.extractLsb, shiftImmediateFieldIdentity]

def shiftImmediateCircuit
    (source : BitVec 64) (shift : BitVec 6) (operation : sop) : BitVec 64 :=
  match operation with
  | .SLLI => source.shiftLeft shift.toNat
  | .SRLI => source.ushiftRight shift.toNat
  | .SRAI => source.sshiftRight shift.toNat

theorem officialShiftImmediateCircuitEquivalence
    (source : BitVec 64) (shift : BitVec 6) (operation : sop) :
    shiftImmediateSemanticValue source shift operation =
      shiftImmediateCircuit source shift operation := by
  cases operation <;>
    simp [shiftImmediateSemanticValue, shiftImmediateCircuit,
      Sail.shift_bits_left, Sail.shift_bits_right, shift_bits_right_arith,
      Sail.BitVec.toNatInt]

#print axioms officialExecuteShiftImmediateBridge
#print axioms officialShiftImmediateCircuitEquivalence
