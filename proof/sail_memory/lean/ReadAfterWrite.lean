-- Prove byte read-after-write against the upstream Sail state monad.
-- Never replace imported memory operations or instruction semantics.
import Sail.ConcurrencyInterfaceV1

open Sail.ConcurrencyInterfaceV1

namespace Hyperray.SailMemory

variable {Register : Type} {RegisterType : Register → Type}
variable [DecidableEq Register] [Hashable Register]
variable {choice : ChoiceSource} {UserError : Type}

theorem read_after_write (state : SequentialState RegisterType choice)
    (address : Nat) (value : BitVec 8) :
    ((do
      PreSail.writeByte address value
      PreSail.readByte address) : PreSailM RegisterType choice UserError (BitVec 8)) state =
    .ok value { state with mem := state.mem.insert address value } := by
  simp only [PreSail.writeByte, PreSail.readByte, bind, EStateM.bind,
    modify, modifyGet, MonadStateOf.modifyGet, EStateM.modifyGet,
    get, getThe, MonadStateOf.get, EStateM.get,
    Std.ExtHashMap.get?_eq_getElem?, Std.ExtHashMap.getElem?_insert_self]
  rfl

/-- info: 'Hyperray.SailMemory.read_after_write' depends on axioms: [propext, Classical.choice, Quot.sound] -/
#guard_msgs in
#print axioms read_after_write

end Hyperray.SailMemory
