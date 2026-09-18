(* Lower the rewritten typed Sail tree with the pinned Sail C backend. *)
(* Return JIB before the backend writes C source. *)
open Libsail

module Codegen = C_backend.Codegen (struct
  let includes = []
  let header_includes = []
  let no_main = true
  let no_lib = false
  let no_rts = false
  let no_mangle = false
  let reserved_words = Util.StringSet.empty
  let overrides = Name_generator.Overrides.empty
  let branch_coverage = None
  let assert_to_exception = false
  let preserve_types = Ast_compare.IdSet.empty
  let cpp = false
  let cpp_class_name = "Model"
  let cpp_namespace = "model"
  let cpp_derive_from = None
end)

let lower state =
  Codegen.jib_of_ast state.Interactive.State.env state.effect_info state.ast
  |> fst
