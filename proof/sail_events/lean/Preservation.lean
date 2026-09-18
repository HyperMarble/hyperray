-- Prove event and finite-choice preservation against imported Sail definitions.
-- These identities never establish whole-program coverage.
import Sail.ArchSem

open Sail.ArchSem

namespace Hyperray.SailEvents

variable [Arch] {UserError : Type} {Value Next : Type}

theorem choice_continuation (count : Nat)
    (continuation : Fin count → PreSailM UserError Value) :
    (PreSail.choose_fin count >>= continuation) =
      FreeM.impure (.inr count) continuation := by
  rfl

theorem effect_continuation (effect : InstructionEffect)
    (continuation : Effect.ret effect → PreSailM UserError Value)
    (next : Value → PreSailM UserError Next) :
    ((FreeM.impure (.inl (.ok effect)) continuation) >>= next) =
      FreeM.impure (.inl (.ok effect))
        (fun reply => continuation reply >>= next) := by
  rfl

theorem barrier_continuation (barrier : Arch.barrier)
    (continuation : Unit → PreSailM UserError Value) :
    (PreSail.sail_barrier barrier >>= continuation) =
      FreeM.impure (.inl (.ok (.barrier barrier))) continuation := by
  rfl

theorem bitvector_choices (width : Nat) :
    (PreSail.undefined_bitvector width : PreSailM UserError (BitVec width)) =
      FreeM.impure (.inr (2 ^ width : Nat))
        (fun choice : Fin (2 ^ width) => FreeM.pure (BitVec.ofFin choice)) := by
  rfl

theorem barrier_not_pure (barrier : Arch.barrier) :
    (PreSail.sail_barrier barrier : PreSailM UserError Unit) ≠ FreeM.pure () := by
  intro equality
  cases equality

/-- info: 'Hyperray.SailEvents.choice_continuation' does not depend on any axioms -/
#guard_msgs in
#print axioms choice_continuation
/-- info: 'Hyperray.SailEvents.effect_continuation' does not depend on any axioms -/
#guard_msgs in
#print axioms effect_continuation
/-- info: 'Hyperray.SailEvents.barrier_continuation' does not depend on any axioms -/
#guard_msgs in
#print axioms barrier_continuation
/-- info: 'Hyperray.SailEvents.bitvector_choices' does not depend on any axioms -/
#guard_msgs in
#print axioms bitvector_choices
/-- info: 'Hyperray.SailEvents.barrier_not_pure' does not depend on any axioms -/
#guard_msgs in
#print axioms barrier_not_pure

end Hyperray.SailEvents
