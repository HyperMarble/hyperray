(* Hold constructor kinds from one JIB traversal. *)
(* Never store program-specific values or instruction names. *)
open Libsail
open Jib

module KindSet = Set.Make (String)

type t = {
  origins : Jib_origin.t;
  definitions : KindSet.t ref;
  instructions : KindSet.t ref;
  values : KindSet.t ref;
  operations : KindSet.t ref;
  places : KindSet.t ref;
  types : KindSet.t ref;
  initializers : KindSet.t ref;
  type_initializers : KindSet.t ref;
  call_types : KindSet.t ref;
  call_returns : KindSet.t ref;
  function_returns : KindSet.t ref;
  type_definitions : KindSet.t ref;
}

let empty () =
  let fresh () = ref KindSet.empty in
  {
    origins = Jib_origin.create ();
    definitions = fresh (); instructions = fresh (); values = fresh ();
    operations = fresh (); places = fresh (); types = fresh ();
    initializers = fresh (); type_initializers = fresh ();
    call_types = fresh (); call_returns = fresh ();
    function_returns = fresh ();
    type_definitions = fresh ();
  }

let add destination value = destination := KindSet.add value !destination

let record result destination category kind =
  add destination kind;
  Jib_origin.add result.origins category kind

let collect_instruction result instruction =
  let I_aux (node, _) = instruction in
  record result result.instructions "instruction" (Jib_node_kind.of_instruction node);
  match node with
  | I_init (_, _, value) ->
      record result result.initializers "initializer" (Jib_value_kind.of_initializer value)
  | I_funcall (returned, call_type, _, _) ->
      record result result.call_returns "call_return" (Jib_value_kind.of_return returned);
      record result result.call_types "call_type" (Jib_value_kind.of_call_type call_type)
  | _ -> ()

let collect_definition result definition =
  let CDEF_aux (node, _) = definition in
  record result result.definitions "definition" (Jib_node_kind.of_definition node);
  match node with
  | CDEF_type ((CTD_abstract (_, _, init)) as value) ->
      record result result.type_definitions "type_definition" (Jib_value_kind.of_type_definition value);
      record result result.type_initializers "type_initializer" (Jib_value_kind.of_type_initializer init)
  | CDEF_type value ->
      record result result.type_definitions "type_definition" (Jib_value_kind.of_type_definition value)
  | CDEF_fundef (_, name, _, _) ->
      record result result.function_returns "function_return" (Jib_value_kind.of_return_name name)
  | _ -> ()

let values source = KindSet.elements !source
