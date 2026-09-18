(* Visit each JIB node and record its fixed grammar constructor. *)
(* Never change the visited JIB tree. *)
open Libsail
open Jib

let record = Jib_census_data.record

class collector result = object
  inherit Jib_visitor.empty_jib_visitor

  method! vctyp value =
    record result result.Jib_census_data.types "type" (Jib_type_kind.of_type value);
    DoChildren

  method! vclexp value =
    record result result.places "place" (Jib_value_kind.of_place value);
    DoChildren

  method! vcval value =
    record result result.values "value" (Jib_value_kind.of_value value);
    (match value with
    | V_call (operation, _) ->
        record result result.operations "operation" (Jib_type_kind.of_operation operation)
    | _ -> ());
    DoChildren

  method! vinstr value =
    Jib_census_data.collect_instruction result value;
    DoChildren

  method! vcdef value =
    Jib_census_data.collect_definition result value;
    DoChildren
end
