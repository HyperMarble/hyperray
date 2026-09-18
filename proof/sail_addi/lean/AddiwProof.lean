-- Prove that the official ADDIW operation agrees with its bit circuit.
-- Never use this theorem as evidence for another instruction family.
import OfficialADDISlice

open OfficialADDISlice.Functions

def addiwSemanticValue (source : BitVec 64) (immediate : BitVec 12) : BitVec 64 :=
  let result := source + Sail.BitVec.signExtend immediate 64
  Sail.BitVec.signExtend (Sail.BitVec.extractLsb result 31 0) 64

def addiwStateAction
    (immediate : BitVec 12) (sourceRegister destination : regidx) :
    SailM ExecutionResult := do
  let source ← rX_bits sourceRegister
  wX_bits destination (addiwSemanticValue source immediate)
  pure RETIRE_SUCCESS

theorem officialExecuteAddiwBridge
    (immediate : BitVec 12) (sourceRegister destination : regidx) :
    execute_ADDIW immediate sourceRegister destination =
      addiwStateAction immediate sourceRegister destination := by
  simp [execute_ADDIW, addiwStateAction, addiwSemanticValue, sign_extend]

def addiwCircuit (source : BitVec 64) (immediate : BitVec 12) : BitVec 64 :=
  let result := source + immediate.signExtend 64
  (result.extractLsb 31 0).signExtend 64

theorem officialAddiwCircuitEquivalence
    (source : BitVec 64) (immediate : BitVec 12) :
    addiwSemanticValue source immediate = addiwCircuit source immediate := by
  simp [addiwSemanticValue, addiwCircuit, Sail.BitVec.signExtend,
    Sail.BitVec.extractLsb]

#print axioms officialExecuteAddiwBridge
#print axioms officialAddiwCircuitEquivalence
