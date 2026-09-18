-- Prove the official upper-immediate and integer-immediate catalogs.
-- Every family theorem takes a value from this complete catalog.
import OfficialADDISlice

open OfficialADDISlice.Functions

theorem officialITypeCatalog (operation : iop) :
    operation = .ADDI ∨ operation = .SLTI ∨ operation = .SLTIU ∨
      operation = .XORI ∨ operation = .ORI ∨ operation = .ANDI := by
  cases operation <;> simp

theorem officialUTypeCatalog (operation : uop) :
    operation = .LUI ∨ operation = .AUIPC := by
  cases operation <;> simp

theorem officialRTypeCatalog (operation : rop) :
    operation = .ADD ∨ operation = .SUB ∨ operation = .SLL ∨
      operation = .SLT ∨ operation = .SLTU ∨ operation = .XOR ∨
      operation = .SRL ∨ operation = .SRA ∨ operation = .OR ∨
      operation = .AND := by
  cases operation <;> simp

theorem officialShiftImmediateCatalog (operation : sop) :
    operation = .SLLI ∨ operation = .SRLI ∨ operation = .SRAI := by
  cases operation <;> simp

#print axioms officialITypeCatalog
#print axioms officialUTypeCatalog
#print axioms officialRTypeCatalog
#print axioms officialShiftImmediateCatalog
