-- Prove that the official register-register family agrees with bit circuits.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice.ITypeProof

open OfficialADDISlice.Functions

def rTypeSemanticValue
    (source1 source2 : BitVec 64) (operation : rop) : BitVec 64 :=
  let shift := Sail.BitVec.extractLsb source2 5 0
  match operation with
  | .ADD => source1 + source2
  | .SLT => Sail.BitVec.zeroExtend (bool_to_bit (zopz0zI_s source1 source2)) 64
  | .SLTU => Sail.BitVec.zeroExtend (bool_to_bit (zopz0zI_u source1 source2)) 64
  | .AND => source1 &&& source2
  | .OR => source1 ||| source2
  | .XOR => source1 ^^^ source2
  | .SLL => Sail.shift_bits_left source1 shift
  | .SRL => Sail.shift_bits_right source1 shift
  | .SUB => source1 - source2
  | .SRA => shift_bits_right_arith source1 shift

def rTypeStateAction
    (source2Register source1Register destination : regidx)
    (operation : rop) : SailM ExecutionResult := do
  let source1 ← rX_bits source1Register
  let source2 ← rX_bits source2Register
  wX_bits destination (rTypeSemanticValue source1 source2 operation)
  pure RETIRE_SUCCESS

theorem officialExecuteRTypeBridge
    (source2Register source1Register destination : regidx) (operation : rop) :
    execute_RTYPE source2Register source1Register destination operation =
      rTypeStateAction source2Register source1Register destination operation := by
  cases operation <;>
    simp [execute_RTYPE, rTypeStateAction, rTypeSemanticValue, zero_extend,
      OfficialADDISlice.Functions.log2_xlen]

theorem rTypeShiftAmountBridge (source : BitVec 64) :
    (Sail.BitVec.toNatInt (source.extractLsb 5 0)).toNat =
      source.toNat % 64 := by
  simp [Sail.BitVec.toNatInt]
  apply Int.ofNat_inj.mp
  rw [Int.toNat_of_nonneg (Int.emod_nonneg _ (by decide))]
  exact (Int.natCast_emod source.toNat 64).symm

def rTypeCircuit
    (source1 source2 : BitVec 64) (operation : rop) : BitVec 64 :=
  let shift := (source2.extractLsb 5 0).toNat
  match operation with
  | .ADD => source1 + source2
  | .SLT => comparisonCircuit (source1.slt source2)
  | .SLTU => comparisonCircuit (source1.ult source2)
  | .AND => source1 &&& source2
  | .OR => source1 ||| source2
  | .XOR => source1 ^^^ source2
  | .SLL => source1.shiftLeft shift
  | .SRL => source1.ushiftRight shift
  | .SUB => source1 - source2
  | .SRA => source1.sshiftRight shift

theorem officialRTypeCircuitEquivalence
    (source1 source2 : BitVec 64) (operation : rop) :
    rTypeSemanticValue source1 source2 operation =
      rTypeCircuit source1 source2 operation := by
  cases operation <;>
    simp [rTypeSemanticValue, rTypeCircuit, Sail.shift_bits_left,
      Sail.shift_bits_right,
      shift_bits_right_arith, comparisonCircuitBridge, signedComparisonBridge,
      unsignedComparisonBridge, Sail.BitVec.extractLsb]
  rw [rTypeShiftAmountBridge]

#print axioms officialExecuteRTypeBridge
#print axioms officialRTypeCircuitEquivalence
