(* Name the fixed JIB value and storage constructors. *)
(* Never inspect a program-specific identifier or literal. *)
open Libsail
open Jib
[@@@warning "+8"]
[@@@warnerror "+8"]

let of_value = function
  | V_id _ -> "identifier"
  | V_member _ -> "member"
  | V_lit _ -> "literal"
  | V_tuple _ -> "tuple"
  | V_struct _ -> "struct"
  | V_ctor_kind _ -> "constructor_kind"
  | V_ctor_unwrap _ -> "constructor_unwrap"
  | V_tuple_member _ -> "tuple_member"
  | V_call _ -> "operation"
  | V_field _ -> "field"

let of_place = function
  | CL_id _ -> "identifier"
  | CL_rmw _ -> "read_modify_write"
  | CL_field _ -> "field"
  | CL_addr _ -> "address"
  | CL_tuple _ -> "tuple"
  | CL_void _ -> "void"

let of_initializer = function
  | Init_cval _ -> "value"
  | Init_static _ -> "static"
  | Init_json_key _ -> "json_key"

let of_call_type = function Call -> "call" | Extern _ -> "external"
let of_return = function CR_one _ -> "one" | CR_multi _ -> "multiple"

let of_type_definition = function
  | CTD_enum _ -> "enum"
  | CTD_struct _ -> "struct"
  | CTD_variant _ -> "variant"
  | CTD_abbrev _ -> "abbreviation"
  | CTD_abstract _ -> "abstract"

let of_type_initializer = function
  | CTDI_instrs _ -> "instructions"
  | CTDI_none -> "none"

let of_return_name = function
  | Return_via _ -> "via_parameter"
  | Return_plain -> "plain"
