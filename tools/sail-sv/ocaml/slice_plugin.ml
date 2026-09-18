(* Select the complete typed-tree dependency slice for one supplied root. *)
(* Never add an instruction rule, program value, or finite bound. *)
open Libsail

let selected_root = ref None

let options =
  [
    ( Flag.create ~prefix:["ast-slice"] ~arg:"identifier" "root",
      Arg.String (fun value -> selected_root := Some value),
      "Select the typed-tree dependency root" );
  ]

let required_root () =
  match !selected_root with
  | Some root -> root
  | None ->
      raise
        (Reporting.err_general Parse_ast.Unknown
           "The typed-tree dependency root is missing")

let target output_name state =
  Rewrite_route.lower output_name (required_root ()) state

let registration =
  Target.register ~name:"astslice" ~flag:"astslice" ~options
    ~description:"lower one complete typed-tree dependency slice"
    ~skip_initial_rewrite:true ~supports_abstract_types:true
    ~supports_runtime_config:true target
