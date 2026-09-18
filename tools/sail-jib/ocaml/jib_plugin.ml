(* Register the pinned Sail-to-JIB constructor census target. *)
(* Report traversal errors before an artifact exists. *)
open Libsail

let target output_name state =
  let output = Option.value ~default:"sail-jib" output_name ^ ".json" in
  let definitions = Jib_backend.lower state in
  match Jib_census.collect definitions with
  | Ok census -> Jib_output.write output (List.length definitions) census
  | Error message -> raise (Reporting.err_general Parse_ast.Unknown message)

let registration =
  Target.register ~name:"jibcatalog" ~description:"emit used JIB constructors"
    ~rewrites:Jib_rewrites.all ~supports_abstract_types:true
    ~supports_runtime_config:true target
