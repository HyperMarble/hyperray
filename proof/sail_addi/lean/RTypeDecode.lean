-- Prove the official register-register encoder and decoder round trip.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def rTypeInstruction
    (source2 source1 destination : BitVec 5) (operation : rop) : instruction :=
  .RTYPE (.Regidx source2, .Regidx source1, .Regidx destination, operation)

def rTypeEncoding
    (source2 source1 destination : BitVec 5) (operation : rop) : BitVec 32 :=
  encdec_forwards (rTypeInstruction source2 source1 destination operation)

def rTypeFunction7 (operation : rop) : BitVec 7 :=
  match operation with
  | .ADD => 0b0000000 | .SUB => 0b0100000
  | .SLL => 0b0000000 | .SLT => 0b0000000
  | .SLTU => 0b0000000 | .XOR => 0b0000000
  | .SRL => 0b0000000 | .SRA => 0b0100000
  | .OR => 0b0000000 | .AND => 0b0000000

def rTypeFunction3 (operation : rop) : BitVec 3 :=
  match operation with
  | .ADD => 0b000 | .SUB => 0b000
  | .SLL => 0b001 | .SLT => 0b010
  | .SLTU => 0b011 | .XOR => 0b100
  | .SRL => 0b101 | .SRA => 0b101
  | .OR => 0b110 | .AND => 0b111

theorem rTypeEncodedFields
    (source2 source1 destination : BitVec 5) (operation : rop) :
    let encoded := rTypeEncoding source2 source1 destination operation
    Sail.BitVec.extractLsb encoded 31 25 = rTypeFunction7 operation ∧
    Sail.BitVec.extractLsb encoded 24 20 = source2 ∧
    Sail.BitVec.extractLsb encoded 19 15 = source1 ∧
    Sail.BitVec.extractLsb encoded 14 12 = rTypeFunction3 operation ∧
    Sail.BitVec.extractLsb encoded 11 7 = destination ∧
    Sail.BitVec.extractLsb encoded 6 0 = 0b0110011#7 := by
  cases operation <;>
    simp [rTypeEncoding, rTypeInstruction, rTypeFunction7, rTypeFunction3,
      encdec_forwards, encdec_reg_forwards, Sail.BitVec.extractLsb,
      BitVec.extractLsb] <;>
    bv_decide

theorem officialRTypeDecode
    (source2 source1 destination : BitVec 5) (operation : rop) :
    encdec_backwards
        (encdec_forwards
          (rTypeInstruction source2 source1 destination operation)) =
      pure (rTypeInstruction source2 source1 destination operation) := by
  obtain ⟨function7Field, source2Field, source1Field, function3Field,
      destinationField, opcodeField⟩ :=
    rTypeEncodedFields source2 source1 destination operation
  change encdec_backwards
      (rTypeEncoding source2 source1 destination operation) =
    pure (.RTYPE
      (.Regidx source2, .Regidx source1, .Regidx destination, operation))
  cases operation <;>
    simp [encdec_backwards, encdec_uop_backwards_matches,
      encdec_iop_backwards_matches, encdec_reg_backwards,
      encdec_reg_backwards_matches,
      function7Field, source2Field, source1Field, function3Field,
      destinationField, opcodeField, rTypeFunction7, rTypeFunction3]

#print axioms officialRTypeDecode
