-- Confirm that a one-bit ADDI circuit mutation does not pass the proof.
-- The runner requires this file to fail with a counterexample.
import OfficialADDISlice
import Std.Tactic.BVDecide

def addiSemanticValueForMutation
    (source : BitVec 64) (immediate : BitVec 12) : BitVec 64 :=
  source + Sail.BitVec.signExtend immediate 64

def circuitImmediateForMutation (immediate : BitVec 12) : BitVec 64 :=
  if immediate[11] then
    0xfffffffffffff#52 +++ immediate
  else
    0x0000000000000#52 +++ immediate

def mutatedAddiCircuit (source : BitVec 64) (immediate : BitVec 12) : BitVec 64 :=
  (source + circuitImmediateForMutation immediate) ^^^ 1#64

theorem mutatedAddiCircuitEquivalence
    (source : BitVec 64) (immediate : BitVec 12) :
    addiSemanticValueForMutation source immediate =
      mutatedAddiCircuit source immediate := by
  simp only [addiSemanticValueForMutation, mutatedAddiCircuit,
    circuitImmediateForMutation,
    Sail.BitVec.signExtend]
  bv_decide
