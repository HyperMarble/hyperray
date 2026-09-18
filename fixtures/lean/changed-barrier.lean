-- Reject removal of an imported barrier effect.
-- This negative fixture must fail through inequality, not missing imports.
import Sail.ArchSem

open Sail.ArchSem

example [Arch] (barrier : Arch.barrier) :
    (PreSail.sail_barrier barrier : PreSailM Unit Unit) = FreeM.pure () := by
  rfl
