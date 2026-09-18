-- Reject an inclusive-range claim about the unchanged upstream definition.
-- This negative fixture must not conceal the missing upper endpoint.
import Sail.ArchSem

open Sail.ArchSem

example [Arch] :
    (PreSail.undefined_range 0 0 : PreSailM Unit Int) =
      FreeM.impure (.inr (1 : Nat))
        (fun choice : Fin 1 => FreeM.pure ((choice : Nat) : Int)) := by
  rfl
