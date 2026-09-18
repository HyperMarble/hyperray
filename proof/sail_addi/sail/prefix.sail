// Supply finite RV64 state for three official integer instruction families.
// The source script inserts the official enums after line seven.
default Order dec
$include <prelude.sail>

type xlen : Int = 64
type xlenbits = bits(xlen)
type log2_xlen : Int = 6
newtype regidx = Regidx : bits(5)

let xlen = sizeof(xlen)
let log2_xlen = sizeof(log2_xlen)

function bool_to_bit(x : bool) -> bits(1) = if x then 0b1 else 0b0

val sign_extend : forall 'n 'm, 'm >= 'n. (implicit('m), bits('n)) -> bits('m)
function sign_extend(m, v) = sail_sign_extend(v, m)
val zero_extend : forall 'n 'm, 'm >= 'n. (implicit('m), bits('n)) -> bits('m)
function zero_extend(m, v) = sail_zero_extend(v, m)

infix 4 <_s
infix 4 <_u

val operator <_s : forall 'n, 'n > 0. (bits('n), bits('n)) -> bool
val operator <_u : forall 'n. (bits('n), bits('n)) -> bool
function operator <_s (x, y) = signed(x) < signed(y)
function operator <_u (x, y) = unsigned(x) < unsigned(y)

overload operator & = {and_vec}
overload operator | = {or_vec}
overload operator ^ = {xor_vec}

scattered union instruction

union ExecutionResult = {
  Retire_Success : unit,
}

let RETIRE_SUCCESS : ExecutionResult = Retire_Success()

val execute : instruction -> ExecutionResult
scattered function execute

val encdec : instruction <-> bits(32)
scattered mapping encdec

mapping encdec_reg : regidx <-> bits(5) = {
  Regidx(r) <-> r,
}

register Xs : vector(32, dec, xlenbits)
register PC : xlenbits

function get_arch_pc() -> xlenbits = PC

function rX_bits(Regidx(i) : regidx) -> xlenbits =
  if i == 0b00000 then 0x0000000000000000 else Xs[unsigned(i)]

function wX_bits(Regidx(i) : regidx, data : xlenbits) -> unit =
  if i != 0b00000 then Xs[unsigned(i)] = data

overload X = {rX_bits, wX_bits}
