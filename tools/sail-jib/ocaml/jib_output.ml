(* Write the JIB constructor census as deterministic JSON. *)
(* Never emit a completed status without a successful traversal. *)
open Libsail

let strings values = `List (List.map (fun value -> `String value) values)
let kinds source = source |> Jib_census_data.values |> strings
let origins census = census.Jib_census_data.origins |> Jib_origin.values |> List.map Jib_origin.json |> fun values -> `List values

let write path definition_count census =
  let json =
    `Assoc
      [
        ("definition_count", `Int definition_count);
        ("origins", origins census);
        ("definition_kinds", kinds census.Jib_census_data.definitions);
        ("instruction_kinds", kinds census.instructions);
        ("value_kinds", kinds census.values);
        ("operation_kinds", kinds census.operations);
        ("place_kinds", kinds census.places);
        ("type_kinds", kinds census.types);
        ("initializer_kinds", kinds census.initializers);
        ("type_initializer_kinds", kinds census.type_initializers);
        ("call_type_kinds", kinds census.call_types);
        ("call_return_kinds", kinds census.call_returns);
        ("function_return_kinds", kinds census.function_returns);
        ("type_definition_kinds", kinds census.type_definitions);
      ]
  in
  Yojson.Safe.to_file path json
