-- Prove that the complete official integer-immediate family agrees with bit circuits.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice
import Std.Tactic.BVDecide

open OfficialADDISlice.Functions

def iTypeSemanticValue
    (source : BitVec 64) (immediate : BitVec 12) (operation : iop) : BitVec 64 :=
  let immediateValue := Sail.BitVec.signExtend immediate 64
  match operation with
  | .ADDI => source + immediateValue
  | .SLTI => Sail.BitVec.zeroExtend (bool_to_bit (zopz0zI_s source immediateValue)) 64
  | .SLTIU => Sail.BitVec.zeroExtend (bool_to_bit (zopz0zI_u source immediateValue)) 64
  | .ANDI => source &&& immediateValue
  | .ORI => source ||| immediateValue
  | .XORI => source ^^^ immediateValue

def iTypeStateAction
    (immediate : BitVec 12) (sourceRegister destinationRegister : regidx)
    (operation : iop) : SailM ExecutionResult := do
  let sourceValue ← rX_bits sourceRegister
  wX_bits destinationRegister (iTypeSemanticValue sourceValue immediate operation)
  pure RETIRE_SUCCESS

theorem officialExecuteITypeBridge
    (immediate : BitVec 12) (sourceRegister destinationRegister : regidx)
    (operation : iop) :
    execute_ITYPE immediate sourceRegister destinationRegister operation =
      iTypeStateAction immediate sourceRegister destinationRegister operation := by
  cases operation <;>
    simp [execute_ITYPE, iTypeStateAction, iTypeSemanticValue, sign_extend,
      zero_extend]

def comparisonCircuit (condition : Bool) : BitVec 64 :=
  if condition then 1#64 else 0#64

theorem signedComparisonBridge (left right : BitVec 64) :
    zopz0zI_s left right = left.slt right := by
  simp [zopz0zI_s, BitVec.slt]

theorem unsignedComparisonBridge (left right : BitVec 64) :
    zopz0zI_u left right = left.ult right := by
  simp [zopz0zI_u, Sail.BitVec.toNatInt, BitVec.ult]

theorem comparisonCircuitBridge (condition : Bool) :
    Sail.BitVec.zeroExtend (bool_to_bit condition) 64 =
      comparisonCircuit condition := by
  cases condition <;> decide

def iTypeCircuit
    (source : BitVec 64) (immediate : BitVec 12) (operation : iop) : BitVec 64 :=
  let immediateValue := immediate.signExtend 64
  match operation with
  | .ADDI => source + immediateValue
  | .SLTI => comparisonCircuit (source.slt immediateValue)
  | .SLTIU => comparisonCircuit (source.ult immediateValue)
  | .ANDI => source &&& immediateValue
  | .ORI => source ||| immediateValue
  | .XORI => source ^^^ immediateValue

theorem officialITypeCircuitEquivalence
    (source : BitVec 64) (immediate : BitVec 12) (operation : iop) :
    iTypeSemanticValue source immediate operation =
      iTypeCircuit source immediate operation := by
  cases operation <;>
    simp [iTypeSemanticValue, iTypeCircuit,
      comparisonCircuitBridge, signedComparisonBridge,
      unsignedComparisonBridge, Sail.BitVec.signExtend]

#print axioms officialExecuteITypeBridge
#print axioms officialITypeCircuitEquivalence
