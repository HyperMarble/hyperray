(* Name the fixed JIB instruction and definition constructors. *)
(* Never use a default arm that can hide a new constructor. *)
open Libsail
open Jib
[@@@warning "+8"]
[@@@warnerror "+8"]

let of_instruction = function
  | I_decl _ -> "declaration"
  | I_init _ -> "initialization"
  | I_jump _ -> "conditional_jump"
  | I_goto _ -> "goto"
  | I_label _ -> "label"
  | I_funcall _ -> "function_call"
  | I_copy _ -> "copy"
  | I_clear _ -> "clear"
  | I_undefined _ -> "undefined"
  | I_exit _ -> "exit"
  | I_end _ -> "end"
  | I_if _ -> "if"
  | I_block _ -> "block"
  | I_try_block _ -> "try"
  | I_throw _ -> "throw"
  | I_comment _ -> "comment"
  | I_raw _ -> "raw"
  | I_return _ -> "return"
  | I_reset _ -> "reset"
  | I_reinit _ -> "reinitialize"

let of_definition = function
  | CDEF_register _ -> "register"
  | CDEF_type _ -> "type"
  | CDEF_let _ -> "let"
  | CDEF_val _ -> "value_specification"
  | CDEF_fundef _ -> "function"
  | CDEF_startup _ -> "startup"
  | CDEF_finish _ -> "finish"
  | CDEF_pragma _ -> "pragma"
