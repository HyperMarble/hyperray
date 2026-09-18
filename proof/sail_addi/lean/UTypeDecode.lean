-- Prove the official upper-immediate encoder and decoder round trip.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def uTypeInstruction
    (immediate : BitVec 20) (destination : BitVec 5)
    (operation : uop) : instruction :=
  .UTYPE (immediate, .Regidx destination, operation)

theorem officialUTypeDecode
    (immediate : BitVec 20) (destination : BitVec 5) (operation : uop) :
    encdec_backwards
        (encdec_forwards (uTypeInstruction immediate destination operation)) =
      pure (uTypeInstruction immediate destination operation) := by
  let encoded :=
    encdec_forwards (uTypeInstruction immediate destination operation)
  have immediateField : Sail.BitVec.extractLsb encoded 31 12 = immediate := by
    cases operation <;>
      simp [encoded, uTypeInstruction, encdec_forwards, encdec_uop_forwards,
        encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb] <;>
      bv_decide
  have destinationField : Sail.BitVec.extractLsb encoded 11 7 = destination := by
    cases operation <;>
      simp [encoded, uTypeInstruction, encdec_forwards, encdec_uop_forwards,
        encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb] <;>
      bv_decide
  have opcodeField :
      Sail.BitVec.extractLsb encoded 6 0 = encdec_uop_forwards operation := by
    cases operation <;>
      simp [encoded, uTypeInstruction, encdec_forwards, encdec_uop_forwards,
        encdec_reg_forwards, Sail.BitVec.extractLsb, BitVec.extractLsb] <;>
      bv_decide
  have operationDecode :
      encdec_uop_backwards (encdec_uop_forwards operation) = pure operation := by
    cases operation <;> rfl
  have operationMatches :
      encdec_uop_backwards_matches (encdec_uop_forwards operation) = true := by
    cases operation <;> rfl
  change encdec_backwards encoded =
    pure (uTypeInstruction immediate destination operation)
  simp [encdec_backwards, encdec_reg_backwards, encdec_reg_backwards_matches,
    immediateField, destinationField, opcodeField, operationDecode,
    operationMatches, uTypeInstruction]

#print axioms officialUTypeDecode
