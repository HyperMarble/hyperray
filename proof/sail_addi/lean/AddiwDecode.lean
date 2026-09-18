-- Prove the official ADDIW encoder and decoder round trip.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def addiwEncoding
    (immediate : BitVec 12) (source destination : BitVec 5) : BitVec 32 :=
  encdec_forwards (.ADDIW (immediate, .Regidx source, .Regidx destination))

theorem addiwEncodedFields
    (immediate : BitVec 12) (source destination : BitVec 5) :
    let encoded := addiwEncoding immediate source destination
    Sail.BitVec.extractLsb encoded 31 20 = immediate ∧
    Sail.BitVec.extractLsb encoded 19 15 = source ∧
    Sail.BitVec.extractLsb encoded 14 12 = 0b000#3 ∧
    Sail.BitVec.extractLsb encoded 11 7 = destination ∧
    Sail.BitVec.extractLsb encoded 6 0 = 0b0011011#7 := by
  simp [addiwEncoding, encdec_forwards, encdec_reg_forwards,
    Sail.BitVec.extractLsb, BitVec.extractLsb]
  bv_decide

theorem officialAddiwDecode
    (immediate : BitVec 12) (source destination : BitVec 5) :
    encdec_backwards (addiwEncoding immediate source destination) =
      pure (.ADDIW (immediate, .Regidx source, .Regidx destination)) := by
  obtain ⟨immediateField, sourceField, functionField,
      destinationField, opcodeField⟩ :=
    addiwEncodedFields immediate source destination
  simp [encdec_backwards, encdec_uop_backwards_matches,
    encdec_iop_backwards_matches, encdec_reg_backwards,
    encdec_reg_backwards_matches, immediateField, sourceField, functionField,
    destinationField, opcodeField, OfficialADDISlice.Functions.xlen]

#print axioms officialAddiwDecode
