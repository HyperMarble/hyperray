-- Prove that a byte write preserves other read values and errors.
-- Never replace imported memory operations or instruction semantics.
import Sail.ConcurrencyInterfaceV1

open Sail.ConcurrencyInterfaceV1

namespace Hyperray.SailMemory

variable {Register : Type} {RegisterType : Register → Type}
variable [DecidableEq Register] [Hashable Register]
variable {choice : ChoiceSource} {UserError : Type}

theorem read_distinct_address (state : SequentialState RegisterType choice)
    (writeAddress readAddress : Nat) (value : BitVec 8)
    (distinct : writeAddress ≠ readAddress) :
    ((do
      PreSail.writeByte writeAddress value
      PreSail.readByte readAddress) : PreSailM RegisterType choice UserError (BitVec 8)) state =
    match (PreSail.readByte readAddress :
        PreSailM RegisterType choice UserError (BitVec 8)) state with
    | .ok byte _ => .ok byte { state with mem := state.mem.insert writeAddress value }
    | .error error _ => .error error { state with mem := state.mem.insert writeAddress value } := by
  have preserved : (state.mem.insert writeAddress value).get? readAddress =
      state.mem.get? readAddress := by
    simp [Std.ExtHashMap.getElem?_insert, distinct]
  simp only [PreSail.writeByte, PreSail.readByte, bind, EStateM.bind,
    modify, modifyGet, MonadStateOf.modifyGet, EStateM.modifyGet,
    get, getThe, MonadStateOf.get, EStateM.get, preserved]
  cases memoryRead : state.mem.get? readAddress
  all_goals rfl

/-- info: 'Hyperray.SailMemory.read_distinct_address' depends on axioms: [propext, Classical.choice, Quot.sound] -/
#guard_msgs in
#print axioms read_distinct_address

end Hyperray.SailMemory
