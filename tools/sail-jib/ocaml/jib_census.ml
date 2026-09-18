(* Control one complete JIB constructor traversal. *)
(* Return an error if the visitor changes the definition count. *)
open Libsail

let collect definitions =
  let result = Jib_census_data.empty () in
  let visitor = new Jib_collector.collector result in
  let visited = Jib_visitor.visit_cdefs visitor definitions in
  if List.length visited = List.length definitions then Ok result
  else Error "the JIB visitor changed the definition count"
