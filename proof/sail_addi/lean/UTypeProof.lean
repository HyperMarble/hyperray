-- Prove that the official upper-immediate family agrees with bit circuits.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice

open OfficialADDISlice.Functions

def uTypeSemanticValue
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) : BitVec 64 :=
  let offset := Sail.BitVec.signExtend (immediate +++ 0x000#12) 64
  match operation with
  | .LUI => offset
  | .AUIPC => programCounter + offset

def uTypeStateAction
    (immediate : BitVec 20) (destination : regidx)
    (operation : uop) : SailM ExecutionResult :=
  match operation with
  | .LUI => do
      wX_bits destination (uTypeSemanticValue 0#64 immediate .LUI)
      pure RETIRE_SUCCESS
  | .AUIPC => do
      let programCounter ← get_arch_pc ()
      wX_bits destination
        (uTypeSemanticValue programCounter immediate .AUIPC)
      pure RETIRE_SUCCESS

theorem officialExecuteUTypeBridge
    (immediate : BitVec 20) (destination : regidx) (operation : uop) :
    execute_UTYPE immediate destination operation =
      uTypeStateAction immediate destination operation := by
  cases operation <;>
    simp [execute_UTYPE, uTypeStateAction, uTypeSemanticValue, sign_extend]

def uTypeCircuit
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) : BitVec 64 :=
  let upperImmediate := (immediate +++ 0x000#12).signExtend 64
  match operation with
  | .LUI => upperImmediate
  | .AUIPC => programCounter + upperImmediate

theorem officialUTypeCircuitEquivalence
    (programCounter : BitVec 64) (immediate : BitVec 20)
    (operation : uop) :
    uTypeSemanticValue programCounter immediate operation =
      uTypeCircuit programCounter immediate operation := by
  cases operation <;>
    simp [uTypeSemanticValue, uTypeCircuit, Sail.BitVec.signExtend]

#print axioms officialExecuteUTypeBridge
#print axioms officialUTypeCircuitEquivalence
