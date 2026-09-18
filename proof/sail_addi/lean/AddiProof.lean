-- Prove the official integer-immediate encoder and decoder round trip.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def iTypeInstruction
    (immediate : BitVec 12) (source destination : BitVec 5)
    (operation : iop) : instruction :=
  .ITYPE (immediate, .Regidx source, .Regidx destination, operation)

theorem officialITypeDecode
    (immediate : BitVec 12) (source destination : BitVec 5) (operation : iop) :
    encdec_backwards
        (encdec_forwards (iTypeInstruction immediate source destination operation)) =
      pure (iTypeInstruction immediate source destination operation) := by
  let encoded :=
    encdec_forwards (iTypeInstruction immediate source destination operation)
  have immediateField : Sail.BitVec.extractLsb encoded 31 20 = immediate := by
    simp [encoded, iTypeInstruction, encdec_forwards, encdec_iop_forwards,
      encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb]
    bv_decide
  have sourceField : Sail.BitVec.extractLsb encoded 19 15 = source := by
    simp [encoded, iTypeInstruction, encdec_forwards, encdec_iop_forwards,
      encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb]
    bv_decide
  have operationField :
      Sail.BitVec.extractLsb encoded 14 12 = encdec_iop_forwards operation := by
    cases operation <;>
      simp [encoded, iTypeInstruction, encdec_forwards, encdec_iop_forwards,
        encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb] <;>
      bv_decide
  have operationDecode :
      encdec_iop_backwards (encdec_iop_forwards operation) = pure operation := by
    cases operation <;> rfl
  have operationMatches :
      encdec_iop_backwards_matches (encdec_iop_forwards operation) = true := by
    cases operation <;> rfl
  have destinationField : Sail.BitVec.extractLsb encoded 11 7 = destination := by
    simp [encoded, iTypeInstruction, encdec_forwards, encdec_iop_forwards,
      encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb]
    bv_decide
  have opcodeField : Sail.BitVec.extractLsb encoded 6 0 = 0b0010011#7 := by
    simp [encoded, iTypeInstruction, encdec_forwards, encdec_iop_forwards,
      encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb]
    bv_decide
  have uTypeDoesNotMatch : encdec_uop_backwards_matches 0b0010011#7 = false := by
    decide
  change encdec_backwards encoded =
    pure (iTypeInstruction immediate source destination operation)
  simp [encdec_backwards, encdec_reg_backwards, encdec_reg_backwards_matches,
    uTypeDoesNotMatch,
    immediateField, sourceField, operationField, operationDecode,
    operationMatches, destinationField, opcodeField, iTypeInstruction]

#print axioms officialITypeDecode
