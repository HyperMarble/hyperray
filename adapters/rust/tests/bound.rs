// Stage 3 tests share one compiler-MIR fixture set.
// Each case must remain independent of external fixture names.

mod bound_case;
#[path = "bound_tests/inputs.rs"]
mod inputs;
#[path = "bound_tests/loops.rs"]
mod loops;
#[path = "bound_tests/rows.rs"]
mod rows;
