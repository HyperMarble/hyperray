-- Prove the upstream error for an absent byte without a state change.
-- Never replace imported memory operations or instruction semantics.
import Sail.ConcurrencyInterfaceV1

open Sail.ConcurrencyInterfaceV1

namespace Hyperray.SailMemory

variable {Register : Type} {RegisterType : Register → Type}
variable [DecidableEq Register] [Hashable Register]
variable {choice : ChoiceSource} {UserError : Type}

theorem absent_read_error (state : SequentialState RegisterType choice)
    (address : Nat) (absent : state.mem.get? address = none) :
    (PreSail.readByte address : PreSailM RegisterType choice UserError (BitVec 8)) state =
    .error (Sail.Error.OutOfMemoryRange address) state := by
  simp only [PreSail.readByte, bind, EStateM.bind, get, getThe,
    MonadStateOf.get, EStateM.get, absent]
  rfl

/-- info: 'Hyperray.SailMemory.absent_read_error' depends on axioms: [propext, Classical.choice, Quot.sound] -/
#guard_msgs in
#print axioms absent_read_error

end Hyperray.SailMemory
