(* Slice only after Sail applies its complete official rewrite sequence. *)
(* Never replace a Sail rewrite or add machine semantics. *)
open Libsail
open Ast_compare
open Ast_util
open Interactive.State

let official_target () =
  match Target.get ~name:"systemverilog" with
  | Some target -> target
  | None ->
      raise
        (Reporting.err_general Parse_ast.Unknown
           "The Sail SystemVerilog target is missing")

let rewritten_state target state =
  Target.run_pre_rewrites_hook target state.ast state.effect_info state.env;
  let ctx, ast, effect_info, env =
    Rewrites.rewrite state.ctx state.effect_info state.env
      (Target.rewrites target) state.ast
  in
  { state with ctx; ast; effect_info; env }

let sliced_state root state =
  let roots = IdSet.singleton (mk_id root) in
  let ast = Callgraph.filter_ast_ids roots IdSet.empty state.ast in
  { state with ast }

let lower output_name root state =
  let backend = official_target () in
  let rewritten = rewritten_state backend state in
  Target.action backend output_name (sliced_state root rewritten)
