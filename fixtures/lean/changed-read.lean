-- The memory replay must reject this changed read-after-write claim.
-- This negative fixture must never enter an accepted proof module.
import Sail.ConcurrencyInterfaceV1

open Sail.ConcurrencyInterfaceV1

namespace Hyperray.SailMemoryMutation

variable {Register : Type} {RegisterType : Register → Type}
variable [DecidableEq Register] [Hashable Register]
variable {choice : ChoiceSource} {UserError : Type}

theorem changed_read (state : SequentialState RegisterType choice)
    (address : Nat) (value : BitVec 8) :
    ((do
      PreSail.writeByte address value
      PreSail.readByte address) : PreSailM RegisterType choice UserError (BitVec 8)) state =
    .ok (value + 1) { state with mem := state.mem.insert address value } := by
  simp only [PreSail.writeByte, PreSail.readByte, bind, EStateM.bind,
    modify, modifyGet, MonadStateOf.modifyGet, EStateM.modifyGet,
    get, getThe, MonadStateOf.get, EStateM.get,
    Std.ExtHashMap.get?_eq_getElem?, Std.ExtHashMap.getElem?_insert_self]
  rfl

end Hyperray.SailMemoryMutation
