-- Make sure that a one-bit upper-immediate circuit mutation fails.
-- The runner requires a counterexample from this file.
import OfficialADDISlice
import Std.Tactic.BVDecide

def uTypeSemanticValueForMutation
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) : BitVec 64 :=
  let offset := Sail.BitVec.signExtend (immediate +++ 0x000#12) 64
  match operation with
  | .LUI => offset
  | .AUIPC => programCounter + offset

def uTypeCircuitForMutation
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) : BitVec 64 :=
  let upperImmediate := (immediate +++ 0x000#12).signExtend 64
  match operation with
  | .LUI => upperImmediate
  | .AUIPC => programCounter + upperImmediate

def mutatedUTypeCircuit
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) : BitVec 64 :=
  (uTypeCircuitForMutation programCounter immediate operation) ^^^ 1#64

theorem mutatedUTypeCircuitEquivalence
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) :
    uTypeSemanticValueForMutation programCounter immediate operation =
      mutatedUTypeCircuit programCounter immediate operation := by
  cases operation <;>
    simp [uTypeSemanticValueForMutation, mutatedUTypeCircuit,
      uTypeCircuitForMutation,
      Sail.BitVec.signExtend] <;>
    bv_decide
