-- Establish the exact upstream range-choice mismatch without correcting it.
-- An accepted theorem here records a coverage defect, not a proof verdict.
import Sail.ArchSem

open Sail.ArchSem

namespace Hyperray.SailEvents

theorem range_choices [Arch] {UserError : Type} (lower upper : Int) :
    (PreSail.undefined_range lower upper : PreSailM UserError Int) =
      FreeM.impure (.inr (upper - lower).toNat)
        (fun choice : Fin (upper - lower).toNat =>
          FreeM.pure (lower + (choice : Nat))) := by
  rfl

theorem range_excludes_upper (lower upper : Int) (ordered : lower ≤ upper)
    (choice : Fin (upper - lower).toNat) :
    lower + (choice : Nat) < upper := by
  omega

theorem singleton_has_no_reply (bound : Int) :
    ¬Nonempty (Fin (bound - bound).toNat) := by
  rintro ⟨choice⟩
  have bound := choice.isLt
  omega

theorem singleton_not_inclusive [Arch] {UserError : Type} :
    (PreSail.undefined_range 0 0 : PreSailM UserError Int) ≠
      FreeM.impure (.inr (1 : Nat))
        (fun choice : Fin 1 => FreeM.pure ((choice : Nat) : Int)) := by
  intro equality
  cases equality

/-- info: 'Hyperray.SailEvents.range_choices' does not depend on any axioms -/
#guard_msgs in
#print axioms range_choices
/-- info: 'Hyperray.SailEvents.range_excludes_upper' depends on axioms: [propext, Quot.sound] -/
#guard_msgs in
#print axioms range_excludes_upper
/-- info: 'Hyperray.SailEvents.singleton_has_no_reply' depends on axioms: [propext, Quot.sound] -/
#guard_msgs in
#print axioms singleton_has_no_reply
/-- info: 'Hyperray.SailEvents.singleton_not_inclusive' does not depend on any axioms -/
#guard_msgs in
#print axioms singleton_not_inclusive

end Hyperray.SailEvents
