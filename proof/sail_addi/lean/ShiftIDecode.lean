-- Prove the official shift-immediate encoder and decoder round trip.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def shiftImmediateHighBits (operation : sop) : BitVec 6 :=
  match operation with
  | .SLLI => 0b000000 | .SRLI => 0b000000 | .SRAI => 0b010000

def shiftImmediateFunction3 (operation : sop) : BitVec 3 :=
  match operation with
  | .SLLI => 0b001 | .SRLI => 0b101 | .SRAI => 0b101

def shiftImmediateEncoding
    (shift : BitVec 6) (source destination : BitVec 5)
    (operation : sop) : BitVec 32 :=
  encdec_forwards (.SHIFTIOP
    (shift, .Regidx source, .Regidx destination, operation))

theorem shiftImmediateEncodedFields
    (shift : BitVec 6) (source destination : BitVec 5) (operation : sop) :
    let encoded := shiftImmediateEncoding shift source destination operation
    Sail.BitVec.extractLsb encoded 31 26 = shiftImmediateHighBits operation ∧
    Sail.BitVec.extractLsb encoded 25 20 = shift ∧
    Sail.BitVec.extractLsb encoded 19 15 = source ∧
    Sail.BitVec.extractLsb encoded 14 12 = shiftImmediateFunction3 operation ∧
    Sail.BitVec.extractLsb encoded 11 7 = destination ∧
    Sail.BitVec.extractLsb encoded 6 0 = 0b0010011#7 := by
  cases operation <;>
    simp [shiftImmediateEncoding, shiftImmediateHighBits,
      shiftImmediateFunction3, encdec_forwards, encdec_reg_forwards,
      Sail.BitVec.extractLsb, BitVec.extractLsb] <;>
    bv_decide

theorem officialShiftImmediateDecode
    (shift : BitVec 6) (source destination : BitVec 5) (operation : sop) :
    encdec_backwards (shiftImmediateEncoding shift source destination operation) =
      pure (.SHIFTIOP
        (shift, .Regidx source, .Regidx destination, operation)) := by
  obtain ⟨highField, shiftField, sourceField, functionField,
      destinationField, opcodeField⟩ :=
    shiftImmediateEncodedFields shift source destination operation
  cases operation <;>
    simp [encdec_backwards, encdec_uop_backwards_matches,
      encdec_iop_backwards_matches, encdec_reg_backwards,
      encdec_reg_backwards_matches, highField, shiftField, sourceField,
      functionField, destinationField, opcodeField,
      shiftImmediateHighBits, shiftImmediateFunction3,
      OfficialADDISlice.Functions.xlen]

#print axioms officialShiftImmediateDecode
