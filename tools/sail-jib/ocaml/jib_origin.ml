(* Give every visited JIB constructor a stable traversal identity. *)
(* Never derive an identity from an instruction or program name. *)
type entry = { id : string; category : string; kind : string }

type t = { mutable next : int; mutable reversed : entry list }

let create () = { next = 0; reversed = [] }

let add store category kind =
  let id = Printf.sprintf "jib:%08d" store.next in
  store.next <- store.next + 1;
  store.reversed <- { id; category; kind } :: store.reversed

let values store = List.rev store.reversed

let json value =
  `Assoc
    [
      ("origin_id", `String value.id);
      ("category", `String value.category);
      ("kind", `String value.kind);
    ]
